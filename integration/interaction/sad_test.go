package interaction_test

import (
	"errors"
	"testing"
	"time"

	"ai-harness/integration"
	"ai-harness/llm/mocks"

	"github.com/stretchr/testify/mock"
)

// ---------------------------------------------------------------------------
// Consent denial
// ---------------------------------------------------------------------------

func TestToolConsentDenied(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("read the file"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "read_file", `{"file_name":"test.txt"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("Understood, I won't use that tool"), nil).Once()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, testTools)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("read test.txt")
	h.Expect("Allow?", 5*time.Second)
	h.Send("n")
	h.Expect("Understood, I won't use that tool", 10*time.Second)
}

// ---------------------------------------------------------------------------
// Tool not found
// ---------------------------------------------------------------------------

func TestToolNotFound(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("run nonexistent tool"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "nonexistent_tool", `{"arg":"val"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("Tool was not found, noted"), nil).Once()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, testTools)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("run nonexistent tool")
	h.Expect("Allow?", 5*time.Second)
	h.Send("y")
	h.Expect("Tool was not found, noted", 10*time.Second)
}

// ---------------------------------------------------------------------------
// Invalid tool arguments
// ---------------------------------------------------------------------------

func TestToolInvalidArgs(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("read file with bad args"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "read_file", "not-valid-json"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("Args were invalid, will retry"), nil).Once()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, testTools)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("read file with bad args")
	h.Expect("Allow?", 5*time.Second)
	h.Send("y")
	h.Expect("Args were invalid, will retry", 10*time.Second)
}

// ---------------------------------------------------------------------------
// LLM error then recovery
// ---------------------------------------------------------------------------

func TestLLMErrorThenRecovery(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(nil, errors.New("connection timeout")).
		Once()
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(integration.NewStopResponse("recovered!"), nil).
		Once()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	// Turn 1: LLM error → error message shown, prompt reappears
	h.Send("hello")
	h.Expect("LLM API error", 5*time.Second)
	h.Expect("try again later", 5*time.Second)
	h.Expect("ai-harness > ", 5*time.Second)

	// Turn 2: LLM succeeds → content shown
	h.Send("hello again")
	h.Expect("recovered!", 5*time.Second)
}

// ---------------------------------------------------------------------------
// LLM empty choices then recovery
// ---------------------------------------------------------------------------

func TestLLMEmptyChoicesThenRecovery(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	// Client retries empty choices internally — agent only sees the final result
	mockLLM := integration.NewMockLLMWithResponse(t, "now I have content")

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("question one")
	h.Expect("now I have content", 10*time.Second)
}

// ---------------------------------------------------------------------------
// /compact with empty history
// ---------------------------------------------------------------------------

func TestSlashCompactEmptyHistory(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("/compact")
	h.Expect("No chat history to compact.", 3*time.Second)
}

// ---------------------------------------------------------------------------
// Consent empty input treated as allow
// ---------------------------------------------------------------------------

func TestConsentEmptyInputAllowed(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("read the file"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "read_file", `{"file_name":"test.txt"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("file read ok"), nil).Once()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, testTools)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("read test.txt")
	h.Expect("Allow?", 5*time.Second)
	h.Send("") // empty = allow (per tui.go logic)
	h.Expect("file read ok", 10*time.Second)
}

// ---------------------------------------------------------------------------
// Tool denied then new turn works
// ---------------------------------------------------------------------------

func TestToolDeniedThenNewTurn(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("read the file"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "read_file", `{"file_name":"secret.txt"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("Tool denied, understood"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("Here is your answer"), nil).Once()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, testTools)

	h.Expect("ai-harness > ", 3*time.Second)

	// Turn 1: tool denied
	h.Send("read secret.txt")
	h.Expect("Allow?", 5*time.Second)
	h.Send("n")
	h.Expect("Tool denied, understood", 10*time.Second)

	// Turn 2: new message works normally
	h.Send("what is 2+2?")
	h.Expect("Here is your answer", 5*time.Second)
}
