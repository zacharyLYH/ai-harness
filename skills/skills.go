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
	b.WriteString("You have the following skills available as tools. If a skill is relevant to the user's request, call the skill's tool to receive the full instructions. Follow those instructions carefully.\n\n")

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
