package agent

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
