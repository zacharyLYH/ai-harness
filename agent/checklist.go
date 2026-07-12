package agent

import (
	"fmt"
	"strings"

	"ai-harness/common/tui"
	"ai-harness/llm"
)

// ChecklistItem represents a single task in a checklist.
type ChecklistItem struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	SeedContext string `json:"seed_context"`
	Status      string `json:"status"` // "pending" | "in_progress" | "done" | "failed"
	Result      string `json:"result"`
}

// Checklist holds an ordered list of tasks for subagents to execute.
type Checklist struct {
	Items []ChecklistItem `json:"items"`
}

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
		toolAllowlist: a.toolAllowlist,
		skills:        a.skills,
		skillsDir:     a.skillsDir,
		depth:         a.depth + 1,
		turnCount:     0,
	}

	prompt := item.Description
	if item.SeedContext != "" {
		prompt = fmt.Sprintf("%s\n\nContext:\n%s", item.Description, item.SeedContext)
	}

	a.logger.SystemLog("[%s] Spawning subagent: %s", a.ID, subID)
	tui.Infof(a.printPrefix+"\n  🚀 Spawning subagent: %s — %s\n", subID, item.Description)
	return sub.AgenticLoop(prompt, tools)
}

func (a *Agent) executeChecklist(checklist *Checklist, tools []llm.Tool) {
	var previousResults strings.Builder

	for i := range checklist.Items {
		item := &checklist.Items[i]
		item.Status = "in_progress"
		tui.Mutedf(a.printPrefix+"  📌 [%d/%d] %s\n", i+1, len(checklist.Items), item.Description)

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

		tui.Infof(a.printPrefix+"  ✓ [%d/%d] %s — %s\n", i+1, len(checklist.Items), item.Description, item.Status)
	}
}

func (a *Agent) synthesizeChecklistResults(checklist *Checklist, tools []llm.Tool) string {
	var sb strings.Builder
	sb.WriteString("All checklist tasks have been completed. Here are the results:\n\n")
	for _, item := range checklist.Items {
		sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n  Result: %s\n\n", item.Status, item.ID, item.Description, item.Result))
	}
	sb.WriteString("Please provide a final synthesized summary of what was accomplished.")

	a.appendToHistory("User: %s", sb.String())

	messages := a.createMessagesFromHistory()
	response, err := a.llmClient.Chat(messages, nil)
	if err != nil {
		a.logger.SystemLog("[%s] Error synthesizing results: %v", a.ID, err)
		return sb.String()
	}

	choice, err := firstChoice(response)
	if err != nil {
		return sb.String()
	}
	return messageContent(choice.Message.Content)
}
