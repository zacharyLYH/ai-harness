package agent

import (
	"fmt"
	"sort"
	"strings"

	"ai-harness/common/tui"
	"ai-harness/llm"
	"ai-harness/skills"
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
		tui.Print("  /skill <create/add/show/view/delete/edit>   - Create, inspect, edit, or delete skills")
		tui.Print("  /perms    - Show approved tool permissions")
	case "/context":
		a.mu.Lock()
		totalWords := 0
		for _, entry := range a.chatHistory {
			totalWords += len(strings.Fields(entry))
		}
		tui.Printf("  📊 Context: %d words (%d messages)", totalWords, len(a.chatHistory))
		a.mu.Unlock()
	case "/skill", "/skills":
		a.handleSkillCommand(parts[1:])
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

func (a *Agent) handleSkillCommand(args []string) {
	if len(args) == 0 || args[0] == "list" {
		a.listSkills()
		return
	}
	switch args[0] {
	case "create", "add":
		a.addSkill()
	case "show", "view":
		if len(args) < 2 {
			tui.Print("  Usage: /skill show <name>")
			return
		}
		a.showSkill(args[1])
	case "edit":
		if len(args) < 2 {
			tui.Print("  Usage: /skill edit <name>")
			return
		}
		a.editSkill(args[1])
	case "delete":
		if len(args) < 2 {
			tui.Print("  Usage: /skill delete <name>")
			return
		}
		a.deleteSkill(args[1])
	default:
		a.printSkillHelp()
	}
}

func (a *Agent) printSkillHelp() {
	tui.Print("  Skill commands:")
	tui.Print("    /skill list                 List available skills")
	tui.Print("    /skill create               Create a skill (or /skill add)")
	tui.Print("    /skill show <name>          View a skill's instructions")
	tui.Print("    /skill edit <name>          Update a skill")
	tui.Print("    /skill delete <name>        Delete a skill")
}

func (a *Agent) listSkills() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.skills) == 0 {
		tui.Print("  No skills loaded yet. Create one with /skill create.")
		return
	}
	tui.Printf("  Skills (%d):", len(a.skills))
	for _, s := range a.skills {
		tui.Printf("    %s%s%s — %s", tui.Cyan, s.Name, tui.Reset, s.Description)
	}
	tui.Mutedf("  Use /skill show <name> to inspect one, or /skill create to add one.")
}

func (a *Agent) showSkill(name string) {
	name = skills.NormalizeName(name)
	a.mu.Lock()
	skill := skills.FindSkillByName(a.skills, name)
	a.mu.Unlock()
	if skill == nil {
		tui.Printf("  Skill %q not found. Run /skill list to see available skills.", name)
		return
	}

	tui.Print("")
	tui.Infof("  Skill: %s", skill.Name)
	tui.Printf("  %s", skill.Description)
	tui.Sep()
	tui.Print(skill.Instructions)
	tui.Mutedf("  Edit it with /skill edit %s", skill.Name)
}

func (a *Agent) addSkill() {
	tui.Print("")
	tui.Infof("  New Skill")
	tui.Sep()

	name := tui.ReadLine("Name (e.g. code-review): ")
	if name == "" {
		tui.Print("  Cancelled.")
		return
	}
	name = skills.NormalizeName(name)
	if err := skills.ValidateName(name); err != nil {
		tui.PrintErr(err, "invalid skill name")
		return
	}

	a.mu.Lock()
	existing := skills.FindSkillByName(a.skills, name)
	a.mu.Unlock()
	if existing != nil {
		tui.Printf("  Skill %q already exists. Use /skill edit %s", name, name)
		return
	}

	description := tui.ReadLine("Description: ")
	if description == "" {
		tui.Print("  Cancelled.")
		return
	}

	instructions := tui.ReadMultiLine("Instructions (markdown):")
	if instructions == "" {
		tui.Print("  Cancelled.")
		return
	}

	skill := skills.Skill{Name: name, Description: description, Instructions: instructions}

	if err := skills.SaveSkill(a.skillsDir, skill); err != nil {
		tui.PrintErr(err, "saving skill")
		return
	}

	a.mu.Lock()
	a.skills = append(a.skills, skill)
	a.mu.Unlock()

	tui.Printf("  %sSkill %q saved.%s", tui.Green, name, tui.Reset)
	tui.Mutedf("  It is ready to use in your next message. Inspect it with /skill show %s.", name)
}

func (a *Agent) editSkill(name string) {
	name = skills.NormalizeName(name)
	a.mu.Lock()
	existing := skills.FindSkillByName(a.skills, name)
	a.mu.Unlock()

	if existing == nil {
		tui.Printf("  Skill %q not found.", name)
		return
	}

	tui.Print("")
	tui.Infof("  Edit Skill: %s", name)
	tui.Sep()

	tui.Mutedf("  Leave a prompt blank to keep its current value.\n")

	newDesc := tui.ReadLine(fmt.Sprintf("Description [%s]: ", existing.Description))
	if newDesc == "" {
		newDesc = existing.Description
	}

	tui.Mutedf("  Current instructions:\n%s\n", existing.Instructions)
	newInstructions := tui.ReadMultiLine("Replacement instructions")
	if newInstructions == "" {
		newInstructions = existing.Instructions
	}

	updated := skills.Skill{Name: existing.Name, Description: newDesc, Instructions: newInstructions}

	if err := skills.SaveSkill(a.skillsDir, updated); err != nil {
		tui.PrintErr(err, "saving skill")
		return
	}

	a.mu.Lock()
	for i, s := range a.skills {
		if strings.EqualFold(s.Name, name) {
			a.skills[i] = updated
			break
		}
	}
	a.mu.Unlock()

	tui.Printf("  %sSkill %q updated.%s", tui.Green, name, tui.Reset)
}

func (a *Agent) deleteSkill(name string) {
	name = skills.NormalizeName(name)
	a.mu.Lock()
	existing := skills.FindSkillByName(a.skills, name)
	a.mu.Unlock()

	if existing == nil {
		tui.Printf("  Skill %q not found. Run /skill list to see available skills.", name)
		return
	}

	confirm := tui.ReadLine(fmt.Sprintf("Delete %q? [y/N]: ", name))
	if !strings.EqualFold(confirm, "y") {
		tui.Print("  Cancelled.")
		return
	}

	if err := skills.DeleteSkill(a.skillsDir, name); err != nil {
		if strings.Contains(err.Error(), "not found") {
			tui.Printf("  Skill %q is bundled and cannot be deleted. Use /skill edit %s to create a personal override.", name, name)
			return
		}
		tui.PrintErr(err, "deleting skill")
		return
	}

	a.mu.Lock()
	for i, s := range a.skills {
		if strings.EqualFold(s.Name, name) {
			a.skills = append(a.skills[:i], a.skills[i+1:]...)
			break
		}
	}
	a.mu.Unlock()

	tui.Printf("  %sSkill %q deleted.%s", tui.Green, name, tui.Reset)
}
