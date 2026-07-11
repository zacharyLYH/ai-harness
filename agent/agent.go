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

// Agent handles the interactive agent loop, chat history, tool execution and slash commands.
type Agent struct {
	llmClient     llm.Client
	toolManager   *tools.DefaultToolManager
	logger        *logger.Logger
	chatHistory   []string
	toolAllowlist map[string]interface{} // toolAllowlist[toolName+argsSummary]
	skills        []skills.Skill
	mu            sync.Mutex
}

func New(llmClient llm.Client, toolManager *tools.DefaultToolManager, logger *logger.Logger) *Agent {
	return &Agent{
		llmClient:     llmClient,
		toolManager:   toolManager,
		logger:        logger,
		chatHistory:   make([]string, 0),
		toolAllowlist: make(map[string]interface{}),
		skills:        make([]skills.Skill, 0),
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

// AgenticLoop runs the main agent loop: sends messages to LLM, processes tool calls, etc.
func (a *Agent) AgenticLoop(prompt string, tools []llm.Tool) {
	a.logger.SystemLog("User prompt: %s", prompt)

	a.mu.Lock()
	a.chatHistory = append(a.chatHistory, fmt.Sprintf("User: %s", prompt))
	a.mu.Unlock()

	apiTools := convertToolsToAPIFormat(tools)
	// Append skill tool definitions so the LLM can request skill instructions via tool calls
	apiTools = append(apiTools, skills.ToToolDefinitions(a.skills)...)

	messages := a.createMessagesFromHistory()

	for {
		done := make(chan bool, 1)
		go showLoading(done)
		response, err := a.llmClient.Chat(messages, apiTools)
		done <- true
		fmt.Print("\r                                \r")
		if err != nil {
			a.logger.SystemLog("Error calling LLM: %v", err)
			return
		}

		if len(response.Choices) == 0 {
			a.logger.SystemLog("No response from LLM")
			return
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

		a.logger.SystemLog("Finish reason: %s", response.Choices[0].FinishReason)

		if response.Choices[0].FinishReason == "tool_calls" && len(assistantMessage.ToolCalls) > 0 {
			toolCall := assistantMessage.ToolCalls[0]

			// Intercept skill tool calls — return the full instructions as the tool result
			if skill := skills.FindSkillByName(a.skills, toolCall.Function.Name); skill != nil {
				fmt.Printf("\n  ⚙ Loading skill: %s\n", skill.Name)
				a.mu.Lock()
				a.chatHistory = append(a.chatHistory, fmt.Sprintf("System Skill Context: %s", skill.Instructions))
				a.mu.Unlock()

				messages = append(messages, assistantMessage)
				messages = append(messages, llm.Message{
					Role:       "tool",
					Content:    skill.Instructions,
					ToolCallID: toolCall.ID,
				})
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
				continue
			}

			fmt.Printf("\n  ⚙ Running tool: %s\n", toolCall.Function.Name)
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
			continue
		} else {
			printSeparator()
			fmt.Println(assistantContent)
			printSeparator()
			break
		}
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
		if len(a.toolAllowlist) == 0 {
			fmt.Println("  No permissions granted yet.")
		} else {
			fmt.Println("  Approved permissions:")
			for toolName, argsSummary := range a.toolAllowlist {
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
	_, toolExists := a.toolAllowlist[toolName+argsSummary]
	if toolExists {
		a.mu.Unlock()
		return true
	}
	a.mu.Unlock()

	// For bash, use the LLM-provided description, falling back to asking the LLM to explain
	if toolName == "bash" {
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
		fmt.Print("Allow? (y/N): ")
	} else {
		fmt.Printf("\nTool '%s' wants to run with args: %s\nAllow? (y/N): ", toolName, argsSummary)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	answer := strings.TrimSpace(scanner.Text())
	if strings.ToLower(answer) != "y" {
		return false
	}

	// Cache the permission
	a.mu.Lock()
	a.toolAllowlist[toolName+argsSummary] = nil
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

// createMessagesFromHistory converts the chat history to LLM messages,
// injecting skills into the first system message at the beginning of the conversation.
func (a *Agent) createMessagesFromHistory() []llm.Message {
	var messages []llm.Message

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
			// Prepend a system message with skills context
			messages = append(messages, llm.Message{
				Role:    "system",
				Content: skillsPrompt,
			})
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
