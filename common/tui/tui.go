package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Cyan   = "\033[36m"
	Gray   = "\033[90m"
)

type ConsentDecision int

const (
	ConsentDenied ConsentDecision = iota
	ConsentOnce
	ConsentAll
)

func Print(msg string) {
	fmt.Println(msg)
}

func Printf(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func Mutedf(format string, args ...interface{}) {
	Printf(Gray+format+Reset, args...)
}

func Infof(format string, args ...interface{}) {
	Printf(Cyan+format+Reset, args...)
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
				fmt.Print("\r\033[2K")
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

func Consent(toolName, explanation, argsSummary string) (ConsentDecision, error) {
	fmt.Printf("\n  %sPermission required%s · %s\n", Yellow, Reset, toolName)
	if toolName == "bash" && explanation != "" {
		fmt.Printf("  %s%s%s\n", Gray, explanation, Reset)
	} else if argsSummary != "" {
		fmt.Printf("  %s%s%s\n", Gray, argsSummary, Reset)
	}

	fmt.Printf("Allow? [y] once, [a] all %s calls, [N] no: ", toolName)

	scanner := bufio.NewReader(os.Stdin)
	line, err := scanner.ReadString('\n')
	if err != nil {
		return ConsentDenied, err
	}
	answer := strings.TrimSpace(line)
	if strings.EqualFold(answer, "a") {
		return ConsentAll, nil
	}
	if strings.EqualFold(answer, "n") {
		return ConsentDenied, nil
	}
	return ConsentOnce, nil
}
