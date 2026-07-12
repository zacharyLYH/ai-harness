package interaction_test

import (
	"testing"
	"time"

	"ai-harness/integration"
	"ai-harness/llm/mocks"
	"ai-harness/skills"

	"github.com/stretchr/testify/mock"
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
	h.Send("/skills")
	h.Expect("write-a-poem: How to write a poem in my way", 3*time.Second)
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
	h.Send("/skills")
	h.Expect("Loaded skills (2):", 3*time.Second)
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
	h.Send("/skills")
	h.Expect("code-review: Reviews code for security issues", 3*time.Second)
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
