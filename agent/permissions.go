package agent

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"ai-harness/common/tui"
	"ai-harness/llm"
)

func (a *Agent) isToolAllowed(toolName string, argumentsJSON string) bool {
	var args map[string]interface{}
	json.Unmarshal([]byte(argumentsJSON), &args)

	keys := make([]string, 0, len(args))
	for key := range args {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := args[k]
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}

	argsSummary := strings.Join(parts, ", ")
	permissionKey := toolName + "\x00" + argumentsJSON
	if normalized, err := json.Marshal(args); err == nil {
		permissionKey = toolName + "\x00" + string(normalized)
	}

	a.mu.Lock()
	_, toolExists := (*a.toolAllowlist)[permissionKey]
	_, toolAllowed := (*a.toolAllowlist)[toolName+"\x00*"]
	if toolExists || toolAllowed {
		a.mu.Unlock()
		return true
	}
	a.mu.Unlock()

	var explanation string
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

		explanation = description
		if explanation == "" {
			var err error
			explanation, err = a.askForExplanation(command)
			if err != nil {
				explanation = fmt.Sprintf("Executes: %s", command)
			}
		}
	}

	decision, err := tui.Consent(toolName, explanation, argsSummary)
	if err != nil || decision == tui.ConsentDenied {
		return false
	}
	a.mu.Lock()
	if decision == tui.ConsentAll {
		(*a.toolAllowlist)[toolName+"\x00*"] = "all calls approved"
	} else {
		(*a.toolAllowlist)[permissionKey] = argsSummary
	}
	a.mu.Unlock()
	return true
}

func (a *Agent) askForExplanation(command string) (string, error) {
	prompt := fmt.Sprintf(explanationPrompt+"\n\nCommand: %s", command)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}
	response, err := a.llmClient.Chat(messages, nil)
	if err != nil {
		return "", err
	}
	choice, err := firstChoice(response)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(messageContent(choice.Message.Content)), nil
}
