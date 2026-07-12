package integration

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ai-harness/agent"
	"ai-harness/app"
	"ai-harness/common/logger"
	"ai-harness/llm"
	"ai-harness/llm/mocks"
	"ai-harness/skills"
	"ai-harness/tools"

	"github.com/creack/pty"
	"github.com/stretchr/testify/mock"
)

// ConsoleHarness manages a virtual PTY pair for E2E testing.
type ConsoleHarness struct {
	T      *testing.T
	Master *os.File
	Slave  *os.File
	data   chan string
	errc   chan error
}

func NewConsoleHarness(t *testing.T) *ConsoleHarness {
	t.Helper()
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open: %v", err)
	}
	h := &ConsoleHarness{
		T:      t,
		Master: ptmx,
		Slave:  tty,
		data:   make(chan string, 100),
		errc:   make(chan error, 1),
	}
	go h.readLoop()
	return h
}

func (h *ConsoleHarness) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := h.Master.Read(buf)
		if n > 0 {
			h.data <- string(buf[:n])
		}
		if err != nil {
			if err != io.EOF {
				h.errc <- err
			}
			return
		}
	}
}

// Send simulates user input (text + Enter).
func (h *ConsoleHarness) Send(input string) {
	h.T.Helper()
	if _, err := io.WriteString(h.Master, input+"\n"); err != nil {
		h.T.Fatalf("Send: %v", err)
	}
}

// SendRaw writes raw bytes to the PTY (e.g. \x03 for ^C).
func (h *ConsoleHarness) SendRaw(input string) {
	h.T.Helper()
	if _, err := io.WriteString(h.Master, input); err != nil {
		h.T.Fatalf("SendRaw: %v", err)
	}
}

// Expect blocks until the given fragment appears in the PTY output or timeout fires.
func (h *ConsoleHarness) Expect(expected string, timeout time.Duration) {
	h.T.Helper()
	var buf strings.Builder
	deadline := time.After(timeout)
	for {
		select {
		case chunk := <-h.data:
			buf.WriteString(chunk)
			if strings.Contains(buf.String(), expected) {
				return
			}
		case err := <-h.errc:
			h.T.Fatalf("stream error: %v", err)
		case <-deadline:
			h.T.Fatalf("timeout waiting for %q\ncaptured:\n%s", expected, buf.String())
		}
	}
}

func (h *ConsoleHarness) Close() {
	h.Slave.Close()
	h.Master.Close()
}

// NewTestAgent creates a logger + agent wired to the given mock LLM.
func NewTestAgent(t *testing.T, mockLLM *mocks.Client) *agent.Agent {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "test.log")
	log, err := logger.NewLogger(logPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	return agent.New(mockLLM, tools.NewDefaultToolManager(), log, nil)
}

// NewTestAgentWithSkills creates a logger + agent with loaded skills.
func NewTestAgentWithSkills(t *testing.T, mockLLM *mocks.Client, loadedSkills []skills.Skill) *agent.Agent {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "test.log")
	log, err := logger.NewLogger(logPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	return agent.New(mockLLM, tools.NewDefaultToolManager(), log, loadedSkills)
}

// BootLoop starts RunInteractiveLoop in a goroutine and returns a done channel
// that closes when the loop exits. It registers a cleanup to wait for the
// goroutine so it doesn't leak into the next test.
func BootLoop(t *testing.T, h *ConsoleHarness, agt *agent.Agent, tools []llm.Tool) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		app.RunInteractiveLoop(agt, tools, "ai-harness > ", h.Slave)
		close(done)
	}()
	t.Cleanup(func() { <-done })
	return done
}

// ---------------------------------------------------------------------------
// Mock helpers — reduce boilerplate in integration tests.
// ---------------------------------------------------------------------------

// NewMockLLMWithResponse creates a mock LLM that returns a single stop response.
func NewMockLLMWithResponse(t *testing.T, content string) *mocks.Client {
	t.Helper()
	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(&llm.ChatResponse{
			Choices: []llm.Choice{{
				FinishReason: "stop",
				Message:      llm.Message{Content: content},
			}},
		}, nil).
		Once()
	return mockLLM
}

// NewMockLLMWithResponses creates a mock LLM that returns stop responses in sequence.
func NewMockLLMWithResponses(t *testing.T, responses ...string) *mocks.Client {
	t.Helper()
	mockLLM := mocks.NewClient(t)
	for _, resp := range responses {
		content := resp
		mockLLM.EXPECT().
			Chat(mock.Anything, mock.Anything).
			Return(&llm.ChatResponse{
				Choices: []llm.Choice{{
					FinishReason: "stop",
					Message:      llm.Message{Content: content},
				}},
			}, nil).
			Once()
	}
	return mockLLM
}

// NewChecklistBypassResponse returns a tool_calls response with a single-item
// checklist, which the agent treats as a simple task and executes directly.
func NewChecklistBypassResponse(description string) *llm.ChatResponse {
	return &llm.ChatResponse{
		Choices: []llm.Choice{{
			FinishReason: "tool_calls",
			Message: llm.Message{
				ToolCalls: []llm.ToolCall{{
					ID:   "call_0",
					Type: "function",
					Function: llm.ToolCallFunction{
						Name:      "create_checklist",
						Arguments: fmt.Sprintf(`{"items":[{"id":"t1","description":"%s","seed_context":""}]}`, description),
					},
				}},
			},
		}},
	}
}

// NewToolCallResponse returns a tool_calls response for a single tool invocation.
func NewToolCallResponse(callID, name, args string) *llm.ChatResponse {
	return &llm.ChatResponse{
		Choices: []llm.Choice{{
			FinishReason: "tool_calls",
			Message: llm.Message{
				ToolCalls: []llm.ToolCall{{
					ID:   callID,
					Type: "function",
					Function: llm.ToolCallFunction{
						Name:      name,
						Arguments: args,
					},
				}},
			},
		}},
	}
}

// NewStopResponse returns a simple stop response.
func NewStopResponse(content string) *llm.ChatResponse {
	return &llm.ChatResponse{
		Choices: []llm.Choice{{
			FinishReason: "stop",
			Message:      llm.Message{Content: content},
		}},
	}
}
