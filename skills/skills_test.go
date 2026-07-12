package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveSkill(t *testing.T) {
	dir := t.TempDir()
	skill := Skill{
		Name:         "test-skill",
		Description:  "A test skill",
		Instructions: "## Instructions\nDo something",
	}

	err := SaveSkill(dir, skill)
	if err != nil {
		t.Fatalf("SaveSkill: %v", err)
	}

	path := filepath.Join(dir, "test-skill", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "name: test-skill") {
		t.Error("missing name in frontmatter")
	}
	if !strings.Contains(content, "description: A test skill") {
		t.Error("missing description in frontmatter")
	}
	if !strings.Contains(content, "## Instructions") {
		t.Error("missing instructions body")
	}
}

func TestDeleteSkill(t *testing.T) {
	dir := t.TempDir()
	skill := Skill{Name: "to-delete", Description: "desc", Instructions: "body"}

	if err := SaveSkill(dir, skill); err != nil {
		t.Fatalf("SaveSkill: %v", err)
	}

	if err := DeleteSkill(dir, "to-delete"); err != nil {
		t.Fatalf("DeleteSkill: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "to-delete")); !os.IsNotExist(err) {
		t.Error("skill directory should have been removed")
	}
}

func TestDeleteSkill_NotFound(t *testing.T) {
	dir := t.TempDir()
	err := DeleteSkill(dir, "nonexistent")
	if err == nil {
		t.Error("expected error for missing skill")
	}
}

func TestLoadAllSkills_MissingDirectoryIsEmpty(t *testing.T) {
	loaded, err := LoadAllSkills(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("LoadAllSkills: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("expected no skills, got %d", len(loaded))
	}
}

func TestValidateName(t *testing.T) {
	if got := NormalizeName("Code Review"); got != "code-review" {
		t.Errorf("NormalizeName() = %q, want code-review", got)
	}
	for _, name := range []string{"code-review", "review2"} {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q): %v", name, err)
		}
	}
	for _, name := range []string{"../outside", "Code Review", "-review", "review-", "review--code"} {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) succeeded, want error", name)
		}
	}
}

func TestFormatSkillFile(t *testing.T) {
	skill := Skill{
		Name:         "my-skill",
		Description:  "does stuff",
		Instructions: "Step 1\nStep 2",
	}

	content := FormatSkillFile(skill)

	if !strings.HasPrefix(content, "---\n") {
		t.Error("should start with frontmatter delimiter")
	}
	if !strings.Contains(content, "name: my-skill") {
		t.Error("missing name")
	}
	if !strings.Contains(content, "description: does stuff") {
		t.Error("missing description")
	}
	if !strings.Contains(content, "Step 1\nStep 2") {
		t.Error("missing instructions")
	}
}

func TestFormatSkillFile_EscapesYAMLDescription(t *testing.T) {
	skill := Skill{Name: "review", Description: "Review code: security first", Instructions: "Check auth."}
	parsed, err := parseSkillContent(FormatSkillFile(skill))
	if err != nil {
		t.Fatalf("parseSkillContent: %v", err)
	}
	if parsed.Description != skill.Description {
		t.Errorf("description = %q, want %q", parsed.Description, skill.Description)
	}
}

func TestParseSkillContent(t *testing.T) {
	content := "---\nname: parsed-skill\ndescription: test desc\n---\n\nDo the thing"
	skill, err := parseSkillContent(content)
	if err != nil {
		t.Fatalf("parseSkillContent: %v", err)
	}
	if skill.Name != "parsed-skill" {
		t.Errorf("name = %q, want %q", skill.Name, "parsed-skill")
	}
	if skill.Description != "test desc" {
		t.Errorf("description = %q, want %q", skill.Description, "test desc")
	}
	if skill.Instructions != "Do the thing" {
		t.Errorf("instructions = %q, want %q", skill.Instructions, "Do the thing")
	}
}

func TestLoadAllSkills_WithUserDir(t *testing.T) {
	dir := t.TempDir()

	skill := Skill{Name: "loaded-skill", Description: "desc", Instructions: "body"}
	if err := SaveSkill(dir, skill); err != nil {
		t.Fatalf("SaveSkill: %v", err)
	}

	loaded, err := LoadAllSkills(dir)
	if err != nil {
		t.Fatalf("LoadAllSkills: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(loaded))
	}
	if loaded[0].Name != "loaded-skill" {
		t.Errorf("name = %q, want %q", loaded[0].Name, "loaded-skill")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	original := Skill{
		Name:         "round-trip",
		Description:  "tests round trip",
		Instructions: "Line 1\nLine 2\nLine 3",
	}

	if err := SaveSkill(dir, original); err != nil {
		t.Fatalf("SaveSkill: %v", err)
	}

	loaded, err := LoadAllSkills(dir)
	if err != nil {
		t.Fatalf("LoadAllSkills: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(loaded))
	}

	got := loaded[0]
	if got.Name != original.Name {
		t.Errorf("name = %q, want %q", got.Name, original.Name)
	}
	if got.Description != original.Description {
		t.Errorf("description = %q, want %q", got.Description, original.Description)
	}
	if got.Instructions != original.Instructions {
		t.Errorf("instructions = %q, want %q", got.Instructions, original.Instructions)
	}
}

func TestUserSkillsDir(t *testing.T) {
	dir := UserSkillsDir()
	if !strings.Contains(dir, ".ai-harness") {
		t.Errorf("UserSkillsDir should contain .ai-harness, got %q", dir)
	}
	if !strings.HasSuffix(dir, "skills") {
		t.Errorf("UserSkillsDir should end with skills, got %q", dir)
	}
}
