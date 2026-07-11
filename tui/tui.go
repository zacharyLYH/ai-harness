package tui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chzyer/readline"
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Gray   = "\033[90m"
)

func Print(msg string) {
	fmt.Println(msg)
}

func Printf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func PrintErr(err error, msg string) {
	if msg != "" {
		fmt.Fprintf(os.Stderr, Red+"Error:"+Reset+" %s: %v\n", msg, err)
	} else {
		fmt.Fprintf(os.Stderr, Red+"Error:"+Reset+" %v\n", err)
	}
}

func Sep() {
	fmt.Println(strings.Repeat("━", 60))
}

func ShowSpinner(text string) func() {
	done := make(chan bool)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		frames := []string{"◐", "◓", "◑", "◒"}
		i := 0
		for {
			select {
			case <-done:
				fmt.Print("\r                                \r")
				return
			default:
				fmt.Printf("\r  "+Blue+"%s"+Reset+" %s", frames[i%len(frames)], text)
				i++
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
	return func() { close(done); wg.Wait() }
}

func Consent(toolName, explanation, argsSummary string) (bool, error) {
	if toolName == "bash" && explanation != "" {
		fmt.Printf("\nTool '%s' wants to run\n", toolName)
		fmt.Printf("  %s\n", explanation)
	} else if argsSummary != "" {
		fmt.Printf("\nTool '%s' wants to run with args: %s\n", toolName, argsSummary)
	}

	rl, err := readline.New("Allow? " + Yellow + "(y/N)" + Reset + ": ")
	if err != nil {
		return false, err
	}
	defer rl.Close()

	line, err := rl.Readline()
	if err != nil {
		return false, err
	}
	answer := strings.TrimSpace(line)
	return strings.ToLower(answer) != "n", nil
}
