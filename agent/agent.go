package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"ai-harness/common/logger"
	"ai-harness/llm"
	"ai-harness/skills"
	"ai-harness/tools"
)

var checklistToolDefinition = llm.ToolDefinition{
	Type: "function",
	Function: llm.ToolFunction{
		Name:        "create_checklist",
		Description: "Mandatory first step to plan the execution of the user's request. Create a checklist of subtasks. If the task is simple and doesn't require subtasks, return an empty array or an array with 1 item.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"items": map[string]interface{}{
					"type":        "array",
					"description": "List of tasks to complete",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id":           map[string]interface{}{"type": "string", "description": "Unique ID for the task"},
							"description":  map[string]interface{}{"type": "string", "description": "What to do"},
							"seed_context": map[string]interface{}{"type": "string", "description": "Starter code or context"},
						},
						"required": []string{"id", "description", "seed_context"},
					},
				},
			},
			"required": []string{"items"},
		},
	},
}

// Agent handles the interactive agent loop, chat history, tool execution and slash commands.
type Agent struct {
	ID            string
	isSubagent    bool
	hasChecklist  bool
	printPrefix   string
	llmClient     llm.Client
	toolManager   *tools.DefaultToolManager
	logger        *logger.Logger
	chatHistory   []string
	toolAllowlist *map[string]interface{} // shared pointer so subagents inherit permissions
	skills        []skills.Skill
	mu            sync.Mutex
	depth         int
	turnCount     int
}

func New(llmClient llm.Client, toolManager *tools.DefaultToolManager, logger *logger.Logger) *Agent {
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
		skills:        make([]skills.Skill, 0),
		depth:         0,
		turnCount:     0,
	}
}

// SetSkills sets the skills that the agent should inject into every prompt turn.
func (a *Agent) SetSkills(skillsList []skills.Skill) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.skills = skillsList
}

// SetChatHistory sets the chat history (used for testing).
func (a *Agent) SetChatHistory(history []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.chatHistory = history
}

// ChatHistory returns a copy of the current chat history (used for testing).
func (a *Agent) ChatHistory() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	cp := make([]string, len(a.chatHistory))
	copy(cp, a.chatHistory)
	return cp
}

// agentPrint prints with the agent's prefix (empty for parent, "[sub:ID]" for subagents).
func (a *Agent) agentPrint(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if a.printPrefix != "" {
		fmt.Print(a.printPrefix + msg)
	} else {
		fmt.Print(msg)
	}
}

// agentPrintln is agentPrint with a trailing newline.
func (a *Agent) agentPrintln(msg string) {
	if a.printPrefix != "" {
		fmt.Println(a.printPrefix + msg)
	} else {
		fmt.Println(msg)
	}
}

// AgenticLoop runs the main agent loop: sends messages to LLM, processes tool calls, etc.
// Returns the final assistant response text.
func (a *Agent) AgenticLoop(prompt string, tools []llm.Tool) string {
	a.turnCount++
	a.logger.LogTurnStart(a.turnCount)
	a.logger.LogUserPrompt(prompt)

	a.mu.Lock()
	a.chatHistory = append(a.chatHistory, fmt.Sprintf("User: %s", prompt))
	a.mu.Unlock()

	apiTools := convertToolsToAPIFormat(tools)
	// Append skill tool definitions so the LLM can request skill instructions via tool calls
	apiTools = append(apiTools, skills.ToToolDefinitions(a.skills)...)

	messages := a.createMessagesFromHistory()

	for {
		// Only show loading spinner for parent agents
		var done chan bool
		if !a.isSubagent {
			done = make(chan bool, 1)
			go showLoading(done)
		}

		var currentTools []llm.ToolDefinition
		if !a.isSubagent && !a.hasChecklist {
			currentTools = []llm.ToolDefinition{checklistToolDefinition}
		} else {
			currentTools = apiTools
		}

		response, err := a.llmClient.Chat(messages, currentTools)
		if done != nil {
			done <- true
			fmt.Print("\r                                \r")
		}
		if err != nil {
			a.logger.SystemLog("[%s] Error calling LLM: %v", a.ID, err)
			return ""
		}

		if len(response.Choices) == 0 {
			a.logger.SystemLog("[%s] No response from LLM", a.ID)
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

		a.mu.Lock()
		a.chatHistory = append(a.chatHistory, fmt.Sprintf("Assistant: %s", assistantContent))
		a.mu.Unlock()

		a.logger.LogLLMMessage(assistantContent)

		if response.Choices[0].FinishReason == "tool_calls" && len(assistantMessage.ToolCalls) > 0 {
			toolCall := assistantMessage.ToolCalls[0]
			a.logger.LogToolCall(toolCall.Function.Name, toolCall.Function.Arguments)

			// Handle checklist tool creation
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

					a.agentPrint("\n  📋 Checklist detected with %d items\n", len(checklist.Items))
					for _, item := range checklist.Items {
						a.agentPrint("    - %s\n", item.Description)
					}
					a.logger.SystemLog("[%s] Checklist created: %+v", a.ID, checklist)

					a.executeChecklist(checklist, tools)

					summary := a.synthesizeChecklistResults(checklist, tools)
					printSeparator()
					fmt.Println(summary)
					printSeparator()
					a.logger.LogTurnEnd()
					return summary
				} else {
					a.agentPrint("\n  🚀 Simple task detected. Executing directly...\n")
					a.logger.SystemLog("[%s] Simple task bypass", a.ID)

					messages = append(messages, assistantMessage)
					messages = append(messages, llm.Message{
						Role:       "tool",
						Content:    "Checklist accepted. Task is simple. Proceed directly with standard tools.",
						ToolCallID: toolCall.ID,
					})

					a.logger.LogToolResult(toolCall.Function.Name, "Checklist accepted. Task is simple. Proceed directly with standard tools.")
					a.logger.LogTurnEnd()
					a.turnCount++
					a.logger.LogTurnStart(a.turnCount)
					continue
				}
			}

			// Intercept skill tool calls — return the full instructions as the tool result
			if skill := skills.FindSkillByName(a.skills, toolCall.Function.Name); skill != nil {
				a.agentPrint("\n  ⚙ Loading skill: %s\n", skill.Name)
				a.mu.Lock()
				a.chatHistory = append(a.chatHistory, fmt.Sprintf("System Skill Context: %s", skill.Instructions))
				a.mu.Unlock()

				messages = append(messages, assistantMessage)
				messages = append(messages, llm.Message{
					Role:       "tool",
					Content:    skill.Instructions,
					ToolCallID: toolCall.ID,
				})

				a.logger.LogToolResult(toolCall.Function.Name, skill.Instructions)
				a.logger.LogTurnEnd()
				a.turnCount++
				a.logger.LogTurnStart(a.turnCount)
				continue
			}

			if !a.isToolAllowed(toolCall.Function.Name, toolCall.Function.Arguments) {
				deniedMsg := fmt.Sprintf("Tool '%s' was not allowed by the user. Do not use this tool again unless the user explicitly asks you to.", toolCall.Function.Name)
				a.mu.Lock()
				a.chatHistory = append(a.chatHistory, fmt.Sprintf("Tool %s denied by user", toolCall.Function.Name))
				a.mu.Unlock()

				messages = append(messages, assistantMessage)
				messages = append(messages, llm.Message{
					Role:       "tool",
					Content:    deniedMsg,
					ToolCallID: toolCall.ID,
				})

				a.logger.LogToolResult(toolCall.Function.Name, deniedMsg)
				a.logger.LogTurnEnd()
				a.turnCount++
				a.logger.LogTurnStart(a.turnCount)
				continue
			}

			a.agentPrint("\n  ⚙ Running tool: %s\n", toolCall.Function.Name)
			toolResult := a.toolManager.Execute(toolCall.Function.Name, toolCall.Function.Arguments, tools)

			a.mu.Lock()
			a.chatHistory = append(a.chatHistory, fmt.Sprintf("Tool %s executed: %s", toolCall.Function.Name, toolResult))
			a.mu.Unlock()

			messages = append(messages, assistantMessage)
			messages = append(messages, llm.Message{
				Role:       "tool",
				Content:    toolResult,
				ToolCallID: toolCall.ID,
			})

			a.logger.LogToolResult(toolCall.Function.Name, toolResult)
			a.logger.LogTurnEnd()
			a.turnCount++
			a.logger.LogTurnStart(a.turnCount)
			continue
		} else {
			// Regular response (no checklist, or subagent)
			if !a.isSubagent {
				printSeparator()
				fmt.Println(assistantContent)
				printSeparator()
			} else {
				a.agentPrintln("  ✅ Done")
			}
			a.logger.LogTurnEnd()
			return assistantContent
		}
	}
}

// spawnSubagent creates a new subagent with a minimal context and runs the given task.
func (a *Agent) spawnSubagent(item ChecklistItem, tools []llm.Tool) string {
	subID := fmt.Sprintf("sub-%s", item.ID)

	sub := &Agent{
		ID:            subID,
		isSubagent:    true,
		printPrefix:   fmt.Sprintf("  [%s] ", subID),
		llmClient:     a.llmClient,
		toolManager:   a.toolManager,
		logger:        a.logger.WithScope(subID, a.depth+1),
		chatHistory:   make([]string, 0),
		toolAllowlist: a.toolAllowlist, // shared pointer
		skills:        a.skills,
		depth:         a.depth + 1,
		turnCount:     0,
	}

	// Build subagent prompt: task description + seed context
	prompt := item.Description
	if item.SeedContext != "" {
		prompt = fmt.Sprintf("%s\n\nContext:\n%s", item.Description, item.SeedContext)
	}

	a.logger.SystemLog("[%s] Spawning subagent: %s", a.ID, subID)
	a.agentPrint("\n  🚀 Spawning subagent: %s — %s\n", subID, item.Description)
	result := sub.AgenticLoop(prompt, tools)
	return result
}

// executeChecklist runs each checklist item sequentially via subagents.
func (a *Agent) executeChecklist(checklist *Checklist, tools []llm.Tool) {
	var previousResults strings.Builder

	for i := range checklist.Items {
		item := &checklist.Items[i]
		item.Status = "in_progress"
		a.agentPrint("  📌 [%d/%d] %s\n", i+1, len(checklist.Items), item.Description)

		// Inject previous results to provide context for this subagent
		if previousResults.Len() > 0 {
			if item.SeedContext != "" {
				item.SeedContext = fmt.Sprintf("Results from previous steps:\n%s\nTask Specific Context:\n%s", previousResults.String(), item.SeedContext)
			} else {
				item.SeedContext = fmt.Sprintf("Results from previous steps:\n%s", previousResults.String())
			}
		}

		result := a.spawnSubagent(*item, tools)
		if result != "" {
			item.Status = "done"
			item.Result = result
			previousResults.WriteString(fmt.Sprintf("- Step %d (%s): %s\n", i+1, item.Description, result))
		} else {
			item.Status = "failed"
			item.Result = "subagent returned empty result"
			previousResults.WriteString(fmt.Sprintf("- Step %d (%s): FAILED\n", i+1, item.Description))
		}

		a.agentPrint("  ✓ [%d/%d] %s — %s\n", i+1, len(checklist.Items), item.Description, item.Status)
	}
}

// synthesizeChecklistResults sends checklist results back to the LLM for a final summary.
func (a *Agent) synthesizeChecklistResults(checklist *Checklist, tools []llm.Tool) string {
	var sb strings.Builder
	sb.WriteString("All checklist tasks have been completed. Here are the results:\n\n")
	for _, item := range checklist.Items {
		sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n  Result: %s\n\n", item.Status, item.ID, item.Description, item.Result))
	}
	sb.WriteString("Please provide a final synthesized summary of what was accomplished.")

	// Send summary to LLM for final response
	a.mu.Lock()
	a.chatHistory = append(a.chatHistory, fmt.Sprintf("User: %s", sb.String()))
	a.mu.Unlock()

	messages := a.createMessagesFromHistory()
	response, err := a.llmClient.Chat(messages, nil)
	if err != nil {
		a.logger.SystemLog("[%s] Error synthesizing results: %v", a.ID, err)
		return sb.String()
	}

	if len(response.Choices) == 0 {
		return sb.String()
	}

	content := response.Choices[0].Message.Content
	switch v := content.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// HandleSlashCommands processes slash commands like /help, /context, /compact.
func (a *Agent) HandleSlashCommands(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "/help":
		fmt.Println("Available commands:")
		fmt.Println("  /context  - Show context size (word/message count)")
		fmt.Println("  /compact  - Compress chat history via LLM")
		fmt.Println("  /help     - Show this help")
		fmt.Println("  /skills   - Show loaded skills")
		fmt.Println("  /perms    - Show approved tool permissions")
	case "/context":
		a.mu.Lock()
		totalWords := 0
		for _, entry := range a.chatHistory {
			totalWords += len(strings.Fields(entry))
		}
		fmt.Printf("  📊 Context: %d words (%d messages)\n", totalWords, len(a.chatHistory))
		a.mu.Unlock()
	case "/skills":
		a.mu.Lock()
		if len(a.skills) == 0 {
			fmt.Println("  No skills loaded.")
		} else {
			fmt.Printf("  Loaded skills (%d):\n", len(a.skills))
			for _, s := range a.skills {
				fmt.Printf("    - %s: %s\n", s.Name, s.Description)
			}
		}
		a.mu.Unlock()
	case "/perms":
		a.mu.Lock()
		if len(*a.toolAllowlist) == 0 {
			fmt.Println("  No permissions granted yet.")
		} else {
			fmt.Println("  Approved permissions:")
			for toolName, argsSummary := range *a.toolAllowlist {
				if argsSummary != "" {
					fmt.Printf("    - %s (%s)\n", toolName, argsSummary)
				} else {
					fmt.Printf("    - %s\n", toolName)
				}
			}
		}
		a.mu.Unlock()
	case "/compact":
		a.mu.Lock()
		if len(a.chatHistory) == 0 {
			fmt.Println("  No chat history to compact.")
			a.mu.Unlock()
			return
		}
		fullText := strings.Join(a.chatHistory, "\n")
		a.mu.Unlock()

		done := make(chan bool, 1)
		go showLoading(done)
		compressed, err := a.compactHistory(fullText)
		done <- true
		fmt.Print("\r                                \r")
		if err != nil {
			fmt.Printf("  Error compacting: %v\n", err)
			return
		}
		a.mu.Lock()
		a.chatHistory = []string{fmt.Sprintf("System: Compressed context — %s", compressed)}
		a.mu.Unlock()
		fmt.Printf("  ✅ Compressed to %d words.\n", len(strings.Fields(compressed)))
	default:
		fmt.Printf("  Unknown command: %s\n", parts[0])
	}
}

// compactHistory sends chat history to the LLM for compression and returns the compressed summary.
func (a *Agent) compactHistory(text string) (string, error) {
	messages := []llm.Message{
		{Role: "system", Content: "You are a chat history compressor. Compress the following conversation into a concise summary that preserves all key information, decisions, and context. Return ONLY the compressed text, no explanations."},
		{Role: "user", Content: text},
	}
	response, err := a.llmClient.Chat(messages, nil)
	if err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}
	content := response.Choices[0].Message.Content
	switch v := content.(type) {
	case string:
		return v, nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// isToolAllowed checks if a tool call is allowed by the user.
// toolAllowlist is a map[toolName]argsSummary — once approved, the tool is cached.
// For the "bash" tool, we ask the LLM to explain the command before prompting the user.
func (a *Agent) isToolAllowed(toolName string, argumentsJSON string) bool {
	var args map[string]interface{}
	json.Unmarshal([]byte(argumentsJSON), &args)

	// Build a human-readable summary of the arguments
	parts := []string{}
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}

	argsSummary := strings.Join(parts, ", ")

	a.mu.Lock()
	_, toolExists := (*a.toolAllowlist)[toolName+argsSummary]
	if toolExists {
		a.mu.Unlock()
		return true
	}
	a.mu.Unlock()

	// For bash, use the LLM-provided description, falling back to asking the LLM to explain
	switch toolName {
	case "bash":
		command := ""
		description := ""
		if args != nil {
			if c, ok := args["command"].(string); ok {
				command = c
			}
			if d, ok := args["description"].(string); ok {
				description = d
			}
		}

		explanation := description
		if explanation == "" {
			var err error
			explanation, err = a.askForExplanation(command)
			if err != nil {
				explanation = fmt.Sprintf("Executes: %s", command)
			}
		}

		fmt.Printf("\nTool '%s' wants to run\n", toolName)
		fmt.Printf("  %s\n", explanation)
		fmt.Print("Allow? (y/N. default yes): ")
	case "curl_web":
		// no-op
	default:
		fmt.Printf("\nTool '%s' wants to run with args: %s\nAllow? (y/N): ", toolName, argsSummary)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	answer := strings.TrimSpace(scanner.Text())
	if strings.ToLower(answer) == "n" {
		return false
	}

	// Cache the permission
	a.mu.Lock()
	(*a.toolAllowlist)[toolName+argsSummary] = nil
	a.mu.Unlock()
	return true
}

// askForExplanation asks the LLM to explain what a bash command does in plain english.
func (a *Agent) askForExplanation(command string) (string, error) {
	prompt := fmt.Sprintf(
		"Explain the following bash command in one short, succinct sentence. "+
			"Do not leave out any important detail like flags, arguments, or side effects.\n\nCommand: %s", command)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}
	response, err := a.llmClient.Chat(messages, nil)
	if err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}
	content := response.Choices[0].Message.Content
	switch v := content.(type) {
	case string:
		return strings.TrimSpace(v), nil
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v)), nil
	}
}

// checklistSystemPrompt is injected for parent agents to instruct the LLM about checklists.
const checklistSystemPrompt = `You must use the create_checklist tool as your very first action.
If the user's task solution is multi-step or multi-instruction and benefits from decomposition into subtasks, provide a list of items.
Each checklist item will be executed by a separate subagent with only the description and seed_context as input.`

// subagentSystemPrompt is injected for subagents.
const subagentSystemPrompt = "You are a focused subagent. Complete the following task thoroughly and respond with your final result. Do not create checklists."

// createMessagesFromHistory converts the chat history to LLM messages,
// injecting skills and agent-role-specific system prompts.
func (a *Agent) createMessagesFromHistory() []llm.Message {
	var messages []llm.Message

	// Inject role-specific system prompt
	if a.isSubagent {
		messages = append(messages, llm.Message{
			Role:    "system",
			Content: subagentSystemPrompt,
		})
	} else {
		// Inject skills as a system message at the start of the conversation
		if skillsPrompt := skills.ToSystemPrompt(a.skills); skillsPrompt != "" {
			// We don't want to keep injecting skills on every turn; we add it once
			// by checking if a skill system message already exists in history.
			hasSkillsMessage := false
			for _, entry := range a.chatHistory {
				if strings.HasPrefix(entry, "System Skill Context: ") {
					hasSkillsMessage = true
					break
				}
			}
			if !hasSkillsMessage {
				messages = append(messages, llm.Message{
					Role:    "system",
					Content: skillsPrompt + "\n\n" + checklistSystemPrompt,
				})
			}
		} else {
			// No skills, but still inject checklist prompt
			hasChecklistMessage := false
			for _, entry := range a.chatHistory {
				if strings.Contains(entry, "checklist") {
					hasChecklistMessage = true
					break
				}
			}
			if !hasChecklistMessage {
				messages = append(messages, llm.Message{
					Role:    "system",
					Content: checklistSystemPrompt,
				})
			}
		}
	}

	for _, entry := range a.chatHistory {
		if strings.HasPrefix(entry, "User: ") {
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: strings.TrimPrefix(entry, "User: "),
			})
		} else if strings.HasPrefix(entry, "Assistant: ") {
			messages = append(messages, llm.Message{
				Role:    "assistant",
				Content: strings.TrimPrefix(entry, "Assistant: "),
			})
		} else if strings.HasPrefix(entry, "System Skill Context: ") {
			// Skip skill context entries in history, we handle them above
			continue
		} else {
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: entry,
			})
		}
	}
	return messages
}

// convertToolsToAPIFormat converts our internal tools to OpenRouter tool definitions.
func convertToolsToAPIFormat(tools []llm.Tool) []llm.ToolDefinition {
	apiTools := make([]llm.ToolDefinition, len(tools))
	for i, tool := range tools {
		apiTools[i] = llm.ToolDefinition{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        tool.ToolName,
				Description: tool.Description,
				Parameters:  tool.Params,
			},
		}
	}
	return apiTools
}

func printSeparator() {
	fmt.Println(strings.Repeat("━", 60))
}

func showLoading(done chan bool) {
	frames := []string{"◐", "◓", "◑", "◒"}
	i := 0
	for {
		select {
		case <-done:
			return
		default:
			fmt.Printf("\r  %s Thinking...", frames[i%len(frames)])
			i++
			time.Sleep(100 * time.Millisecond)
		}
	}
}
