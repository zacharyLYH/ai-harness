package interaction_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"ai-harness/integration"
	"ai-harness/llm"
	"ai-harness/llm/mocks"
	"ai-harness/skills"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var testSkills = []skills.Skill{
	{
		Name:         "write-a-poem",
		Description:  "How to write a poem in my way",
		Instructions: "Use old english\nBe humorous\nEvery line starts with P",
	},
}

var testSkillsMultiple = []skills.Skill{
	{
		Name:         "write-a-poem",
		Description:  "How to write a poem in my way",
		Instructions: "Use old english\nBe humorous\nEvery line starts with P",
	},
	{
		Name:         "summarize-text",
		Description:  "Summarize any text into a short paragraph",
		Instructions: "Read the input carefully\nExtract key points\nOutput a 2-3 sentence summary",
	},
}

func TestSkillCallLoadsInstructions(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("write a poem"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "write-a-poem", `{}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("Ponderous poems produced per preference"), nil).Once()

	agt := integration.NewTestAgentWithSkills(t, mockLLM, testSkills)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("write me a poem about cats")
	h.Expect("Ponderous poems produced per preference", 10*time.Second)
}

func TestSkillAppearsInSlashSkills(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgentWithSkills(t, mockLLM, testSkills)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("/skill")
	h.Expect("write-a-poem", 3*time.Second)
}

func TestMultipleSkillsLoaded(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("summarize text"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "summarize-text", `{}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("Summary complete"), nil).Once()

	agt := integration.NewTestAgentWithSkills(t, mockLLM, testSkillsMultiple)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("summarize this article")
	h.Expect("Summary complete", 10*time.Second)
}

func TestSlashSkillsShowsLoadedCount(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgentWithSkills(t, mockLLM, testSkillsMultiple)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("/skill")
	h.Expect("Skills (2):", 3*time.Second)
}

func TestSkillSkipsConsent(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("write a poem"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "write-a-poem", `{}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("Poem complete"), nil).Once()

	agt := integration.NewTestAgentWithSkills(t, mockLLM, testSkills)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("write a poem")
	h.Expect("Poem complete", 10*time.Second)
}

func TestSkillWithSpecialCharInstructions(t *testing.T) {
	specialSkills := []skills.Skill{
		{
			Name:         "code-review",
			Description:  "Reviews code for security issues",
			Instructions: "Check for SQL injection, XSS, CSRF & other OWASP Top 10. Flag `eval()` usage. Report severity: CRITICAL/HIGH/MEDIUM/LOW",
		},
	}

	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgentWithSkills(t, mockLLM, specialSkills)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("/skill")
	h.Expect("code-review", 3*time.Second)
}

func TestSkillCalledInSecondTurn(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("Hello!"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("write a poem"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "write-a-poem", `{}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("A fine poem"), nil).Once()

	agt := integration.NewTestAgentWithSkills(t, mockLLM, testSkills)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("hi")
	h.Expect("Hello!", 5*time.Second)

	h.Send("write a poem")
	h.Expect("A fine poem", 10*time.Second)
}

func TestSkillList_ShowsLoadedSkills(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgentWithSkills(t, mockLLM, testSkillsMultiple)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("/skill")
	h.Expect("Skills (2):", 3*time.Second)
	h.Expect("write-a-poem", 3*time.Second)
	h.Expect("summarize-text", 3*time.Second)
}

func TestSkillShow_DisplaysInstructions(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgentWithSkills(t, mockLLM, testSkills)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("/skill show write-a-poem")
	h.Expect("Skill: write-a-poem", 3*time.Second)
	h.Expect("Use old english", 3*time.Second)
	h.Expect("/skill edit write-a-poem", 3*time.Second)
}

func TestSkillAdd_CreatesSkillOnDisk(t *testing.T) {
	skillsDir := t.TempDir()

	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgentWithSkillsDir(t, mockLLM, nil, skillsDir)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("/skill add")
	h.Expect("Name", 3*time.Second)

	h.Send("new-skill")
	h.Expect("Description", 3*time.Second)

	h.Send("A brand new skill")
	h.Expect("multi-line", 3*time.Second)

	h.Send("Do this")
	h.Send("Then that")
	h.Send("")
	h.Expect("saved", 5*time.Second)

	// Verify file on disk
	path := filepath.Join(skillsDir, "new-skill", "SKILL.md")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), "name: new-skill")
	require.Contains(t, string(data), "A brand new skill")
	require.Contains(t, string(data), "Do this\nThen that")
}

func TestSkillAdd_DuplicateRejected(t *testing.T) {
	skillsDir := t.TempDir()
	// Pre-populate with a skill
	require.NoError(t, skills.SaveSkill(skillsDir, skills.Skill{
		Name: "existing", Description: "already here", Instructions: "body",
	}))

	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	loaded := []skills.Skill{{Name: "existing", Description: "already here", Instructions: "body"}}
	agt := integration.NewTestAgentWithSkillsDir(t, mockLLM, loaded, skillsDir)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("/skill add")
	h.Expect("Name", 3*time.Second)

	h.Send("existing")
	h.Expect("already exists", 3*time.Second)
}

func TestSkillDelete_RemovesSkill(t *testing.T) {
	skillsDir := t.TempDir()
	require.NoError(t, skills.SaveSkill(skillsDir, skills.Skill{
		Name: "to-delete", Description: "bye", Instructions: "body",
	}))

	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	loaded := []skills.Skill{{Name: "to-delete", Description: "bye", Instructions: "body"}}
	agt := integration.NewTestAgentWithSkillsDir(t, mockLLM, loaded, skillsDir)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("/skill delete to-delete")
	h.Expect("Delete", 3*time.Second)

	h.Send("y")
	h.Expect("deleted", 5*time.Second)

	// Verify file removed from disk
	_, err := os.Stat(filepath.Join(skillsDir, "to-delete"))
	require.True(t, os.IsNotExist(err))
}

func TestSkillDelete_NotFound(t *testing.T) {
	skillsDir := t.TempDir()

	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgentWithSkillsDir(t, mockLLM, nil, skillsDir)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("/skill delete nonexistent")
	h.Expect("not found", 3*time.Second)
}

func TestSkillPersistAcrossReload(t *testing.T) {
	skillsDir := t.TempDir()

	// Create a skill directly on disk
	require.NoError(t, skills.SaveSkill(skillsDir, skills.Skill{
		Name: "persisted", Description: "survives", Instructions: "remember me",
	}))

	// Load skills from disk (simulating app restart)
	loaded, err := skills.LoadAllSkills(skillsDir)
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	require.Equal(t, "persisted", loaded[0].Name)
	require.Equal(t, "survives", loaded[0].Description)
	require.Equal(t, "remember me", loaded[0].Instructions)

	// Create agent with loaded skills and verify they appear
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgentWithSkillsDir(t, mockLLM, loaded, skillsDir)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("/skill")
	h.Expect("persisted", 3*time.Second)
	h.Expect("survives", 3*time.Second)
}

func TestSkillAdd_IncludedInNextPrompt(t *testing.T) {
	skillsDir := t.TempDir()

	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgentWithSkillsDir(t, mockLLM, nil, skillsDir)
	integration.BootLoop(t, h, agt, nil)

	// Step 1: Add a skill
	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("/skill add")
	h.Expect("Name", 3*time.Second)
	h.Send("code-review")
	h.Expect("Description", 3*time.Second)
	h.Send("Reviews code for issues")
	h.Expect("multi-line", 3*time.Second)
	h.Send("Check for bugs")
	h.Send("Check for style")
	h.Send("")
	h.Expect("saved", 5*time.Second)

	// Step 2: Send a prompt — verify the LLM receives the skill as a tool
	h.Expect("ai-harness > ", 3*time.Second)

	// Checklist bypass for the prompt
	mockLLM.EXPECT().Chat(
		mock.Anything,
		mock.MatchedBy(func(tools []llm.ToolDefinition) bool {
			for _, tool := range tools {
				if tool.Function.Name == "code-review" {
					return true
				}
			}
			return false
		}),
	).Return(integration.NewChecklistBypassResponse("review code"), nil).Once()

	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("Reviewed!"), nil).Once()

	h.Send("review my code")
	h.Expect("Reviewed!", 10*time.Second)
}
