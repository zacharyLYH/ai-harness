package app

import (
	"io"
	"os"
	"strings"

	"ai-harness/agent"
	"ai-harness/llm"

	"github.com/chzyer/readline"
)

// RunInteractiveLoop runs the shared readline + agent loop.
// When rw is non-nil, I/O is redirected through it (for PTY integration tests).
// When rw is nil, standard os.Stdin/os.Stdout are used.
func RunInteractiveLoop(agt *agent.Agent, tools []llm.Tool, prompt string, rw io.ReadWriter) {
	if rw != nil {
		origStdout, origStderr := os.Stdout, os.Stderr
		os.Stdout = rw.(*os.File)
		os.Stderr = rw.(*os.File)
		defer func() {
			os.Stdout = origStdout
			os.Stderr = origStderr
		}()
	}

	cfg := &readline.Config{
		Prompt:          prompt,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	}
	if rw != nil {
		cfg.Stdin = rw.(io.ReadCloser)
		cfg.Stdout = rw
		cfg.Stderr = rw
		cfg.ForceUseInteractive = true
	}

	rl, err := readline.NewEx(cfg)
	if err != nil {
		return
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			if rw == nil && err == readline.ErrInterrupt {
				break
			}
			return
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}
		if strings.HasPrefix(input, "/") {
			agt.HandleSlashCommands(input)
			continue
		}
		agt.AgenticLoop(input, tools)
	}
}
