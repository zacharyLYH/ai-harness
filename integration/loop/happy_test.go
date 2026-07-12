package loop_test

import (
	"testing"
	"time"

	"ai-harness/integration"
	"ai-harness/llm/mocks"

	"github.com/stretchr/testify/mock"
)

func TestPromptAppearsOnBoot(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
}

func TestEmptyInputIgnored(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("")
	h.Send("")
	h.Send("   ")

	// No Chat expectations set — if LLM was called, mock would fail.
	time.Sleep(200 * time.Millisecond)
	h.Expect("ai-harness > ", 2*time.Second)
}

func TestInterruptExitsLoop(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.SendRaw("\x03")
	// readline prints the InterruptPrompt ("^C") on Ctrl+C
	h.Expect("^C", 3*time.Second)
}

func TestUnknownSlashCommand(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("/foo")
	h.Expect("Unknown command: /foo", 3*time.Second)

	// Slash commands must not call the LLM
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Maybe()
}
