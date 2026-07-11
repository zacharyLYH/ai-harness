package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"ai-harness/common/tui"
	"ai-harness/llm"
)

func (a *Agent) isToolAllowed(toolName string, argumentsJSON string) bool {
	var args map[string]interface{}
	json.Unmarshal([]byte(argumentsJSON), &args)

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

	allowed, err := tui.Consent(toolName, explanation, argsSummary)
	if err != nil || !allowed {
		return false
	}
	a.mu.Lock()
	(*a.toolAllowlist)[toolName+argsSummary] = nil
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
