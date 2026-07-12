package setup

import (
	"fmt"
	"os"
	"strings"

	"ai-harness/common/tui"
)

const EnvKey = "OPENROUTER_API_KEY"

// KeyURL is where users can grab a free OpenRouter API key.
const KeyURL = "https://openrouter.ai/workspaces/default/keys"

// FileIO abstracts file read/write so it can be mocked in tests.
type FileIO interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
}

// RealFileIO is the production FileIO backed by the os package.
type RealFileIO struct{}

func (RealFileIO) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (RealFileIO) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

// printOnboarding shows the welcome/setup screen when no key is present.
func printOnboarding() {
	tui.Sep()
	tui.Print(tui.Blue + "  Welcome to ai-harness" + tui.Reset)
	tui.Print("  An " + tui.Yellow + "OpenRouter" + tui.Reset + " API key is required to continue.")
	tui.Print("")
	tui.Print("  Get a free key (no card needed):")
	tui.Print("    " + tui.Blue + KeyURL + tui.Reset)
	tui.Sep()
}

// Run ensures an OPENROUTER_API_KEY is available. If it is already present in
// the environment it is returned immediately. Otherwise the user is prompted,
// the supplied testConn is used to validate the key, and the valid key is
// written to .env. It loops until a working key is provided.
//
//   - files:   file I/O (mockable)
//   - prompt:  reads a line of input for the given prompt
//   - testConn: validates an API key, returning nil on success
func Run(files FileIO, prompt func(string) (string, error), testConn func(string) error) (string, error) {
	if key := os.Getenv(EnvKey); key != "" {
		return key, nil
	}

	printOnboarding()

	for {
		raw, err := prompt(tui.Blue + "  Paste your OPENROUTER_API_KEY: " + tui.Reset)
		if err != nil {
			return "", err
		}
		key := strings.TrimSpace(raw)
		if key == "" {
			tui.Print(tui.Red + "  ✗ API key cannot be empty. Please try again." + tui.Reset)
			continue
		}

		stop := tui.ShowSpinner("Testing connection to OpenRouter")
		connErr := testConn(key)
		stop()

		if connErr != nil {
			tui.Print(tui.Red + "  ✗ Connection test failed: " + tui.Reset + connErr.Error())
			tui.Print(tui.Gray + "    Check the key and try again." + tui.Reset)
			continue
		}

		content := fmt.Sprintf("%s=%s\n", EnvKey, key)
		if err := files.WriteFile(".env", []byte(content), 0o644); err != nil {
			return "", err
		}

		tui.Print(tui.Green + "  ✓ Connection successful — saved OPENROUTER_API_KEY to .env" + tui.Reset)
		return key, nil
	}
}
