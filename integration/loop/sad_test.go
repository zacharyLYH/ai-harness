package loop_test

import (
	"errors"
	"testing"
	"time"

	"ai-harness/integration"
	"ai-harness/llm/mocks"

	"github.com/stretchr/testify/mock"
)

func TestLLMCallFailure(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := integration.NewMockLLMWithError(t, errors.New("connection refused"))
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("hello")
	h.Expect("LLM API error", 5*time.Second)
	h.Expect("try again later", 5*time.Second)
	h.Expect("ai-harness > ", 5*time.Second)
}

func TestLLMEmptyChoices(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	// Client retries empty choices internally and returns error after exhausting retries
	mockLLM := integration.NewMockLLMWithError(t, errors.New("LLM API failed after 3 retries: empty choices in LLM response"))
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("hello")
	h.Expect("LLM API error", 5*time.Second)
	h.Expect("try again later", 5*time.Second)
	h.Expect("ai-harness > ", 5*time.Second)
}

func TestReadlineEOF(t *testing.T) {
	h := integration.NewConsoleHarness(t)

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(integration.NewStopResponse("unused"), nil).
		Maybe()
	agt := integration.NewTestAgent(t, mockLLM)
	done := integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	// Close both PTY sides — readline's Read on the slave will error
	h.Slave.Close()
	h.Master.Close()

	select {
	case <-done:
		// goroutine exited cleanly
	case <-time.After(3 * time.Second):
		t.Fatal("goroutine did not exit after PTY close")
	}
}
