package integration_test

import (
	"errors"
	"os"
	"testing"

	"ai-harness/setup"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeFileIO is a mock FileIO for testing setup.Run.
type fakeFileIO struct {
	written   map[string][]byte
	writeErr  error
	readData  []byte
	readErr   error
}

func newFakeFileIO() *fakeFileIO {
	return &fakeFileIO{written: map[string][]byte{}}
}

func (f *fakeFileIO) ReadFile(path string) ([]byte, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	return f.readData, nil
}

func (f *fakeFileIO) WriteFile(path string, data []byte, perm os.FileMode) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.written[path] = data
	return nil
}

func TestRunUsesEnvKeyWithoutPrompting(t *testing.T) {
	t.Setenv(setup.EnvKey, "env-key")

	files := newFakeFileIO()
	promptCalled := false
	connCalled := false

	key, err := setup.Run(files,
		func(string) (string, error) {
			promptCalled = true
			return "", errors.New("prompt should not be called")
		},
		func(string) error {
			connCalled = true
			return nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, "env-key", key)
	assert.False(t, promptCalled, "prompt should not be called when key is in env")
	assert.False(t, connCalled, "connection test should not run when key is in env")
	assert.Empty(t, files.written, ".env should not be written when key is in env")
}

func TestRunPromptsAndWritesEnvOnSuccess(t *testing.T) {
	t.Setenv(setup.EnvKey, "")

	files := newFakeFileIO()

	key, err := setup.Run(files,
		func(string) (string, error) { return "user-provided-key", nil },
		func(apiKey string) error {
			assert.Equal(t, "user-provided-key", apiKey)
			return nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, "user-provided-key", key)
	content, ok := files.written[".env"]
	require.True(t, ok, ".env should be written")
	assert.Equal(t, "OPENROUTER_API_KEY=user-provided-key\n", string(content))
}

func TestRunLoopsUntilConnectionSucceeds(t *testing.T) {
	t.Setenv(setup.EnvKey, "")

	files := newFakeFileIO()
	attempts := 0

	key, err := setup.Run(files,
		func(string) (string, error) { return "k", nil },
		func(string) error {
			attempts++
			if attempts < 3 {
				return errors.New("connection refused")
			}
			return nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, "k", key)
	assert.Equal(t, 3, attempts, "connection test should be retried until success")
	assert.Contains(t, files.written, ".env")
}

func TestRunRejectsEmptyKeyAndRetries(t *testing.T) {
	t.Setenv(setup.EnvKey, "")

	files := newFakeFileIO()
	prompts := []string{"", "valid-key"}
	pi := 0

	key, err := setup.Run(files,
		func(string) (string, error) {
			v := prompts[pi]
			pi++
			return v, nil
		},
		func(apiKey string) error {
			assert.Equal(t, "valid-key", apiKey)
			return nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, "valid-key", key)
	assert.Equal(t, 2, pi, "empty key should trigger a re-prompt")
	assert.Contains(t, files.written, ".env")
}

func TestRunReturnsErrorWhenPromptFails(t *testing.T) {
	t.Setenv(setup.EnvKey, "")

	files := newFakeFileIO()
	wantErr := errors.New("stdin closed")

	_, err := setup.Run(files,
		func(string) (string, error) { return "", wantErr },
		func(string) error { return nil },
	)

	assert.ErrorIs(t, err, wantErr)
	assert.Empty(t, files.written)
}
