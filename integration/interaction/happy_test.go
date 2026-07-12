package interaction_test

import (
	"testing"
	"time"

	"ai-harness/integration"
	"ai-harness/llm"
	"ai-harness/llm/mocks"

	"github.com/stretchr/testify/mock"
)

func TestSingleTurn(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(&llm.ChatResponse{
			Choices: []llm.Choice{{
				FinishReason: "stop",
				Message:      llm.Message{Content: "Hello! How can I help you?"},
			}},
		}, nil).
		Once()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("hello")
	h.Expect("Hello! How can I help you?", 5*time.Second)
}

func TestMultiTurn(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(&llm.ChatResponse{
			Choices: []llm.Choice{{
				FinishReason: "stop",
				Message:      llm.Message{Content: "First response"},
			}},
		}, nil).
		Once()
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(&llm.ChatResponse{
			Choices: []llm.Choice{{
				FinishReason: "stop",
				Message:      llm.Message{Content: "Second response"},
			}},
		}, nil).
		Once()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, nil)

	h.Expect("ai-harness > ", 3*time.Second)

	h.Send("first message")
	h.Expect("First response", 5*time.Second)

	h.Send("second message")
	h.Expect("Second response", 5*time.Second)
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
