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

func TestManyConsecutiveEmptyInputs(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	for i := 0; i < 10; i++ {
		h.Send("")
	}

	time.Sleep(300 * time.Millisecond)
	h.Expect("ai-harness > ", 2*time.Second)
}

func TestWhitespaceOnlyVariations(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("  ")
	h.Send("\t")
	h.Send("   \t  \t  ")
	h.Send("")

	time.Sleep(300 * time.Millisecond)
	h.Expect("ai-harness > ", 2*time.Second)
}

func TestMultipleSlashCommandsInSequence(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("/help")
	h.Expect("Available commands:", 3*time.Second)

	h.Send("/context")
	h.Expect("0 words", 3*time.Second)

	h.Send("/skill")
	h.Expect("No skills loaded", 3*time.Second)
}

func TestSlashCommandWithLeadingWhitespace(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("  /help")
	h.Expect("Available commands:", 3*time.Second)
}

func TestSlashPermsEmpty(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("/perms")
	h.Expect("No permissions granted yet", 3*time.Second)
}
