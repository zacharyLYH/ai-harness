package agent

import (
	"fmt"
	"sort"
	"strings"

	"ai-harness/common/tui"
	"ai-harness/llm"
)

func (a *Agent) HandleSlashCommands(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "/help":
		tui.Print("Available commands:")
		tui.Print("  /context  - Show context size (word/message count)")
		tui.Print("  /compact  - Compress chat history via LLM")
		tui.Print("  /clear    - Clear chat history")
		tui.Print("  /help     - Show this help")
		tui.Print("  /skills   - Show loaded skills")
		tui.Print("  /perms    - Show approved tool permissions")
	case "/context":
		a.mu.Lock()
		totalWords := 0
		for _, entry := range a.chatHistory {
			totalWords += len(strings.Fields(entry))
		}
		tui.Printf("  📊 Context: %d words (%d messages)", totalWords, len(a.chatHistory))
		a.mu.Unlock()
	case "/skills":
		a.mu.Lock()
		if len(a.skills) == 0 {
			tui.Print("  No skills loaded.")
		} else {
			tui.Printf("  Loaded skills (%d):", len(a.skills))
			for _, s := range a.skills {
				tui.Printf("    - %s: %s", s.Name, s.Description)
			}
		}
		a.mu.Unlock()
	case "/perms":
		a.mu.Lock()
		if len(*a.toolAllowlist) == 0 {
			tui.Print("  No permissions granted yet.")
		} else {
			tui.Print("  Approved permissions:")
			keys := make([]string, 0, len(*a.toolAllowlist))
			for key := range *a.toolAllowlist {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				toolName, _, _ := strings.Cut(key, "\x00")
				argsSummary, _ := (*a.toolAllowlist)[key].(string)
				if argsSummary != "" {
					tui.Printf("    - %s (%s)", toolName, argsSummary)
				} else {
					tui.Printf("    - %s", toolName)
				}
			}
		}
		a.mu.Unlock()
	case "/clear":
		a.mu.Lock()
		a.chatHistory = nil
		a.mu.Unlock()
		tui.Print("  ✅ Chat history cleared.")
	case "/compact":
		a.mu.Lock()
		if len(a.chatHistory) == 0 {
			tui.Print("  No chat history to compact.")
			a.mu.Unlock()
			return
		}
		fullText := strings.Join(a.chatHistory, "\n")
		a.mu.Unlock()

		stopSpinner := tui.ShowSpinner("Compressing...")
		compressed, err := a.compactHistory(fullText)
		stopSpinner()
		if err != nil {
			tui.Printf("  Error compacting: %v", err)
			return
		}
		a.mu.Lock()
		a.chatHistory = []string{fmt.Sprintf("System: Compressed context — %s", compressed)}
		a.mu.Unlock()
		tui.Printf("  ✅ Compressed to %d words.", len(strings.Fields(compressed)))
	default:
		tui.Printf("  Unknown command: %s", parts[0])
	}
}

func (a *Agent) compactHistory(text string) (string, error) {
	messages := []llm.Message{
		{Role: "system", Content: compactPrompt},
		{Role: "user", Content: text},
	}
	response, err := a.llmClient.Chat(messages, nil)
	if err != nil {
		return "", err
	}
	choice, err := firstChoice(response)
	if err != nil {
		return "", err
	}
	return messageContent(choice.Message.Content), nil
}
