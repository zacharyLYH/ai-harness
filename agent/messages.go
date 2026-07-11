package agent

import (
	"strings"

	"ai-harness/llm"
	"ai-harness/skills"
)

func (a *Agent) createMessagesFromHistory() []llm.Message {
	var messages []llm.Message

	if a.isSubagent {
		messages = append(messages, llm.Message{
			Role:    "system",
			Content: subagentSystemPrompt,
		})
	} else {
		if skillsPrompt := skills.ToSystemPrompt(a.skills); skillsPrompt != "" {
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
