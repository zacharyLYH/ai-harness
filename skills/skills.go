package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ai-harness/llm"
	"gopkg.in/yaml.v3"
)

// Skill represents a parsed skill from a SKILL.md file.
type Skill struct {
	Name         string
	Description  string
	Instructions string
}

// LoadAllSkills walks the given directory looking for SKILL.md files and parses them.
func LoadAllSkills(skillsDir string) ([]Skill, error) {
	var skills []Skill
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		return skills, nil
	} else if err != nil {
		return nil, fmt.Errorf("checking skills directory %s: %v", skillsDir, err)
	}

	err := filepath.Walk(skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.ToLower(info.Name()) != "skill.md" {
			return nil
		}

		skill, err := ParseSkillFile(path)
		if err != nil {
			return fmt.Errorf("error parsing %s: %v", path, err)
		}
		skills = append(skills, skill)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking skills directory %s: %v", skillsDir, err)
	}

	return skills, nil
}

// ParseSkillFile parses a single SKILL.md file and returns a Skill.
// The file is expected to have YAML frontmatter between --- delimiters,
// followed by the skill instructions in markdown.
func ParseSkillFile(path string) (Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, fmt.Errorf("failed to read %s: %v", path, err)
	}

	content := string(data)
	return parseSkillContent(content)
}

// parseSkillContent parses the content string of a SKILL.md file.
func parseSkillContent(content string) (Skill, error) {
	content = strings.TrimSpace(content)

	if !strings.HasPrefix(content, "---") {
		return Skill{}, fmt.Errorf("missing YAML frontmatter delimiters (---)")
	}

	// Remove the opening ---
	rest := strings.TrimPrefix(content, "---")
	rest = strings.TrimSpace(rest)

	// Find the closing ---
	endIdx := strings.Index(rest, "---")
	if endIdx == -1 {
		return Skill{}, fmt.Errorf("missing closing YAML frontmatter delimiters (---)")
	}

	yamlPart := strings.TrimSpace(rest[:endIdx])
	instructionsPart := strings.TrimSpace(rest[endIdx+3:])

	// Parse YAML frontmatter
	type frontmatter struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(yamlPart), &fm); err != nil {
		return Skill{}, fmt.Errorf("failed to parse YAML frontmatter: %v", err)
	}

	if fm.Name == "" {
		return Skill{}, fmt.Errorf("skill name is required in frontmatter")
	}

	return Skill{
		Name:         fm.Name,
		Description:  fm.Description,
		Instructions: instructionsPart,
	}, nil
}

// ToSystemPrompt converts a list of skills into a system prompt string
// that instructs the LLM about available skills.
func ToSystemPrompt(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("You have the following skills available as tools. If a skill is relevant to the user's request, call the skill's tool to receive the full instructions. Follow those instructions carefully. SKILLS SHOULD USE TOOLS TO PERFORM EXTERNAL SIDE EFFECTS - NEVER PERFORM SIDE EFFECTS FROM SKILLS!!!!!\n\n")

	for i, s := range skills {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, s.Description))
	}

	return b.String()
}

// ToToolDefinitions converts skills into LLM tool definitions so the LLM
// can request skill instructions via a standard tool call.
func ToToolDefinitions(skills []Skill) []llm.ToolDefinition {
	defs := make([]llm.ToolDefinition, 0, len(skills))
	for _, s := range skills {
		defs = append(defs, llm.ToolDefinition{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        s.Name,
				Description: s.Description,
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		})
	}
	return defs
}

// FindSkillByName looks up a skill by name (case-insensitive) and returns a copy.
func FindSkillByName(skills []Skill, name string) *Skill {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, s := range skills {
		if strings.ToLower(s.Name) == name {
			return &s
		}
	}
	return nil
}

// IsSkillTool returns true if the given tool name matches a loaded skill.
func IsSkillTool(toolName string, skills []Skill) bool {
	return FindSkillByName(skills, toolName) != nil
}

// UserSkillsDir returns the path to the user's personal skills directory (~/.ai-harness/skills).
// This directory persists across app reinstalls.
func UserSkillsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ai-harness", "skills")
}

// SaveSkill writes a skill to disk as a SKILL.md file under the given directory.
// It creates the directory structure: <skillsDir>/<name>/SKILL.md
func SaveSkill(skillsDir string, skill Skill) error {
	if err := ValidateName(skill.Name); err != nil {
		return err
	}
	if strings.TrimSpace(skill.Description) == "" {
		return fmt.Errorf("skill description is required")
	}
	skillDir := filepath.Join(skillsDir, skill.Name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("failed to create skill directory: %v", err)
	}

	content := FormatSkillFile(skill)
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write SKILL.md: %v", err)
	}
	return nil
}

// DeleteSkill removes a skill's directory and SKILL.md file.
func DeleteSkill(skillsDir string, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	skillDir := filepath.Join(skillsDir, name)
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		return fmt.Errorf("skill %q not found", name)
	}
	return os.RemoveAll(skillDir)
}

// NormalizeName converts a human-entered title into a skill name.
func NormalizeName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(name)), "-"))
}

// ValidateName ensures a skill name is safe to use as both a tool and directory name.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("skill name is required")
	}
	if len(name) > 63 {
		return fmt.Errorf("skill name must be 63 characters or fewer")
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return fmt.Errorf("skill name %q must use lowercase letters, numbers, and hyphens only", name)
		}
	}
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.Contains(name, "--") {
		return fmt.Errorf("skill name %q cannot start, end, or repeat hyphens", name)
	}
	return nil
}

// FormatSkillFile generates the SKILL.md content for a skill.
func FormatSkillFile(skill Skill) string {
	frontmatter, _ := yaml.Marshal(struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}{Name: skill.Name, Description: skill.Description})
	return "---\n" + string(frontmatter) + "---\n\n" + skill.Instructions
}
