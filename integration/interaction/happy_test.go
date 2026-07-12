package interaction_test

import (
	"testing"
	"time"

	"ai-harness/integration"
	"ai-harness/llm/mocks"

	"github.com/stretchr/testify/mock"
)

func TestSingleTurn(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := integration.NewMockLLMWithResponse(t, "Hello! How can I help you?")
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("hello")
	h.Expect("Hello! How can I help you?", 5*time.Second)
}

func TestMultiTurn(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := integration.NewMockLLMWithResponses(t, "First response", "Second response")
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("first message")
	h.Expect("First response", 5*time.Second)

	h.Send("second message")
	h.Expect("Second response", 5*time.Second)
}

func TestLongInput(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := integration.NewMockLLMWithResponse(t, "Got it")
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	longMsg := "This is a very long message with lots of words. " +
		"The quick brown fox jumps over the lazy dog. " +
		"Pack my box with five dozen liquor jugs. " +
		"How vexingly quick daft zebras jump! " +
		"The five boxing wizards jump quickly."
	h.Send(longMsg)
	h.Expect("Got it", 5*time.Second)
}

func TestLLMEmptyContent(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := integration.NewMockLLMWithResponse(t, "")
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("tell me something")
	// Even with empty content, the loop prints separators and re-prompts
	h.Expect("ai-harness > ", 5*time.Second)
}

func TestLLMVeryLongContent(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	longResp := "This is a substantial response. "
	for i := 0; i < 50; i++ {
		longResp += "The detail goes on and on. "
	}

	mockLLM := integration.NewMockLLMWithResponse(t, longResp)
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("tell me a lot")
	h.Expect("substantial response", 5*time.Second)
	h.Expect("ai-harness > ", 5*time.Second)
}

func TestMixedEmptyInputsAndRealInput(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := integration.NewMockLLMWithResponse(t, "Response")
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("")
	h.Send("")
	h.Send("real input")
	h.Expect("Response", 5*time.Second)
}

func TestEmptyThenSlashThenRealInput(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := integration.NewMockLLMWithResponse(t, "OK")
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("")
	h.Send("/help")
	h.Expect("Available commands:", 3*time.Second)

	h.Send("now talk")
	h.Expect("OK", 5*time.Second)
}

func TestPromptReappearsAfterResponse(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := integration.NewMockLLMWithResponses(t, "First", "Second")
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("one")
	h.Expect("First", 5*time.Second)

	h.Send("two")
	h.Expect("Second", 5*time.Second)
}

func TestLLMMultiLineContent(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := integration.NewMockLLMWithResponse(t, "Line one\nLine two\nLine three")
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("multi line")
	h.Expect("Line one", 5*time.Second)
	h.Expect("ai-harness > ", 5*time.Second)
}

func TestUnicodeUserInput(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := integration.NewMockLLMWithResponse(t, "Unicode response")
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("héllo wörld 你好")
	h.Expect("Unicode response", 5*time.Second)
}

func TestShellSpecialCharactersInInput(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := integration.NewMockLLMWithResponse(t, "Got it")
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("echo $HOME | grep test && echo done")
	h.Expect("Got it", 5*time.Second)
}

func TestSlashCommandBetweenLLMTurns(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := integration.NewMockLLMWithResponses(t, "First answer", "Second answer")
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("question one")
	h.Expect("First answer", 5*time.Second)

	h.Send("/context")
	h.Expect("words", 3*time.Second)

	h.Send("question two")
	h.Expect("Second answer", 5*time.Second)
}

func TestLLMContentWithSpecialChars(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := integration.NewMockLLMWithResponse(t, "Symbols: @#$%^&*() and quotes \"hello\" and path /usr/bin")
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("show me symbols")
	h.Expect("@#$%^&*()", 5*time.Second)
}

func TestRapidSuccessiveInputs(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(integration.NewStopResponse("Reply"), nil).
		Once()
	// readline feeds lines one at a time; "second" may be read after
	// the first turn completes, triggering a second LLM call.
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(integration.NewStopResponse("Reply"), nil).
		Maybe()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("first")
	h.Send("second")
	h.Send("third")
	h.Expect("Reply", 5*time.Second)
}

func TestSlashHelp(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("/help")
	h.Expect("Available commands:", 3*time.Second)
}

func TestSlashContextEmpty(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("/context")
	h.Expect("0 words", 3*time.Second)
}

func TestSlashSkillsEmpty(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("/skills")
	h.Expect("No skills loaded", 3*time.Second)
}
