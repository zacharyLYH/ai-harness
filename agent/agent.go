package agent

import (
	"encoding/json"
	"fmt"
	"sync"

	"ai-harness/common/logger"
	"ai-harness/common/tui"
	"ai-harness/llm"
	"ai-harness/skills"
	toolslib "ai-harness/tools"
)

// Agent handles the interactive agent loop, chat history, tool execution and slash commands.
type Agent struct {
	ID            string
	isSubagent    bool
	hasChecklist  bool
	printPrefix   string
	llmClient     llm.Client
	toolManager   *toolslib.DefaultToolManager
	logger        *logger.Logger
	chatHistory   []string
	toolAllowlist *map[string]interface{}
	skills        []skills.Skill
	mu            sync.Mutex
	depth         int
	turnCount     int
}

func New(llmClient llm.Client, toolManager *toolslib.DefaultToolManager, logger *logger.Logger, loadedSkills []skills.Skill) *Agent {
	allowlist := make(map[string]interface{})
	return &Agent{
		ID:            "root",
		isSubagent:    false,
		printPrefix:   "",
		llmClient:     llmClient,
		toolManager:   toolManager,
		logger:        logger,
		chatHistory:   make([]string, 0),
		toolAllowlist: &allowlist,
		skills:        loadedSkills,
		depth:         0,
		turnCount:     0,
	}
}

func (a *Agent) AgenticLoop(prompt string, tools []llm.Tool) string {
	a.turnCount++
	a.logger.LogTurnStart(a.turnCount)
	a.logger.LogUserPrompt(prompt)
	a.appendToHistory("User: %s", prompt)

	apiTools := convertToolsToAPIFormat(tools)
	apiTools = append(apiTools, skills.ToToolDefinitions(a.skills)...)

	messages := a.createMessagesFromHistory()

	for {
		var stopSpinner func()
		if !a.isSubagent {
			stopSpinner = tui.ShowSpinner("Thinking...")
		}

		var currentTools []llm.ToolDefinition
		if !a.isSubagent && !a.hasChecklist {
			currentTools = []llm.ToolDefinition{toolslib.ChecklistToolDefinition}
		} else {
			currentTools = apiTools
		}

		response, err := a.llmClient.Chat(messages, currentTools)
		if stopSpinner != nil {
			stopSpinner()
		}
		if err != nil {
			a.logger.SystemLog("[%s] Error calling LLM: %v", a.ID, err)
			tui.Printf("Error: LLM API error: %v\nPlease try again later.\n", err)
			a.logger.LogTurnEnd()
			return ""
		}

		assistantMessage := response.Choices[0].Message
		assistantContent := ""

		switch v := assistantMessage.Content.(type) {
		case string:
			assistantContent = v
		case nil:
			assistantContent = ""
		default:
			assistantContent = fmt.Sprintf("%v", v)
		}

		a.appendToHistory("Assistant: %s", assistantContent)
		a.logger.LogLLMMessage(assistantContent)

		if response.Choices[0].FinishReason == "tool_calls" && len(assistantMessage.ToolCalls) > 0 {
			toolCall := assistantMessage.ToolCalls[0]
			a.logger.LogToolCall(toolCall.Function.Name, toolCall.Function.Arguments)

			if toolCall.Function.Name == "create_checklist" {
				a.hasChecklist = true
				defer func() {
					a.hasChecklist = false
				}()
				var args struct {
					Items []ChecklistItem `json:"items"`
				}
				_ = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)

				if len(args.Items) > 1 {
					checklist := &Checklist{Items: args.Items}
					for i := range checklist.Items {
						checklist.Items[i].Status = "pending"
						if checklist.Items[i].ID == "" {
							checklist.Items[i].ID = fmt.Sprintf("task-%d", i+1)
						}
					}

					tui.Printf(a.printPrefix+"\n  📋 Checklist detected with %d items\n", len(checklist.Items))
					for _, item := range checklist.Items {
						tui.Printf(a.printPrefix+"    - %s\n", item.Description)
					}
					a.logger.SystemLog("[%s] Checklist created: %+v", a.ID, checklist)

					a.executeChecklist(checklist, tools)

					summary := a.synthesizeChecklistResults(checklist, tools)
					tui.Sep()
					tui.Print(summary)
					tui.Sep()
					a.logger.LogTurnEnd()
					return summary
				} else {
					tui.Printf(a.printPrefix + "\n  🚀 Simple task detected. Executing directly...\n")
					a.logger.SystemLog("[%s] Simple task bypass", a.ID)
					messages = a.addToolTurn(messages, assistantMessage, toolCall.ID, toolCall.Function.Name, "Checklist accepted. Task is simple. Proceed directly with standard tools.")
					continue
				}
			}

			if skill := skills.FindSkillByName(a.skills, toolCall.Function.Name); skill != nil {
				tui.Printf(a.printPrefix+"\n  ⚙ Loading skill: %s\n", skill.Name)
				a.appendToHistory("System Skill Context: %s", skill.Instructions)
				messages = a.addToolTurn(messages, assistantMessage, toolCall.ID, toolCall.Function.Name, skill.Instructions)
				continue
			}

			if !a.isToolAllowed(toolCall.Function.Name, toolCall.Function.Arguments) {
				deniedMsg := fmt.Sprintf("Tool '%s' was not allowed by the user. Do not use this tool again unless the user explicitly asks you to.", toolCall.Function.Name)
				a.appendToHistory("Tool %s denied by user", toolCall.Function.Name)
				messages = a.addToolTurn(messages, assistantMessage, toolCall.ID, toolCall.Function.Name, deniedMsg)
				continue
			}

			tui.Printf(a.printPrefix+"\n  ⚙ Running tool: %s\n", toolCall.Function.Name)
			toolResult := a.toolManager.Execute(toolCall.Function.Name, toolCall.Function.Arguments, tools)
			a.appendToHistory("Tool %s executed: %s", toolCall.Function.Name, toolResult)
			messages = a.addToolTurn(messages, assistantMessage, toolCall.ID, toolCall.Function.Name, toolResult)
			continue
		} else {
			if !a.isSubagent {
				tui.Sep()
				tui.Print(assistantContent)
				tui.Sep()
			} else {
				tui.Print(a.printPrefix + "  ✅ Done")
			}
			a.logger.LogTurnEnd()
			return assistantContent
		}
	}
}

func (a *Agent) appendToHistory(format string, args ...interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.chatHistory = append(a.chatHistory, fmt.Sprintf(format, args...))
}

func (a *Agent) addToolTurn(messages []llm.Message, assistantMessage llm.Message, toolCallID, toolName, content string) []llm.Message {
	messages = append(messages, assistantMessage, llm.Message{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
	})
	a.logger.LogToolResult(toolName, content)
	a.logger.LogTurnEnd()
	a.turnCount++
	a.logger.LogTurnStart(a.turnCount)
	return messages
}
