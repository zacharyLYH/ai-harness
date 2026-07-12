package setup

import (
	"fmt"
	"os"
	"strings"

	"ai-harness/common/tui"
)

const EnvKey = "OPENROUTER_API_KEY"

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

	for {
		tui.Print("OPENROUTER_API_KEY was not found in the environment.")
		raw, err := prompt("Enter your OPENROUTER_API_KEY: ")
		if err != nil {
			return "", err
		}
		key := strings.TrimSpace(raw)
		if key == "" {
			tui.Print("API key cannot be empty, please try again.")
			continue
		}

		tui.Print("Testing connection to OpenRouter...")
		if err := testConn(key); err != nil {
			tui.PrintErr(err, "connection test failed, please try again")
			continue
		}

		content := fmt.Sprintf("%s=%s\n", EnvKey, key)
		if err := files.WriteFile(".env", []byte(content), 0o644); err != nil {
			return "", err
		}
		tui.Print("Saved OPENROUTER_API_KEY to .env")
		return key, nil
	}
}
