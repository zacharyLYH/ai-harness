package integration

import (
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
	"ai-harness/tools"

	"github.com/creack/pty"
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

// BootLoop starts RunInteractiveLoop in a goroutine and returns a done channel
// that closes when the loop exits.
func BootLoop(t *testing.T, h *ConsoleHarness, agt *agent.Agent, tools []llm.Tool) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		app.RunInteractiveLoop(agt, tools, "ai-harness > ", h.Slave)
		close(done)
	}()
	return done
}
