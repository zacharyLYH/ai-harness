package agent

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-harness/common/logger"
	"ai-harness/llm"
	"ai-harness/llm/mocks"
	"ai-harness/tools"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestAgent(t *testing.T, mockLLM *mocks.Client) *Agent {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "test.log")
	log, err := logger.NewLogger(logPath)
	require.NoError(t, err)
	t.Cleanup(func() { log.Close() })

	if mockLLM == nil {
		mockLLM = mocks.NewClient(t)
	}

	return New(mockLLM, tools.NewDefaultToolManager(), log)
}

// captureOutput executes f and returns everything written to stdout.
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// --- /help tests ---

func TestHandleSlashCommands_Help(t *testing.T) {
	agt := newTestAgent(t, nil)
	output := captureOutput(func() {
		agt.HandleSlashCommands("/help")
	})
	assert.Contains(t, output, "Available commands:")
	assert.Contains(t, output, "/context")
	assert.Contains(t, output, "/compact")
	assert.Contains(t, output, "/help")
}

func TestHandleSlashCommands_EmptyString(t *testing.T) {
	agt := newTestAgent(t, nil)
	output := captureOutput(func() {
		agt.HandleSlashCommands("")
	})
	assert.Empty(t, output)
}

// --- /context tests ---

func TestHandleSlashCommands_Context_Empty(t *testing.T) {
	agt := newTestAgent(t, nil)
	output := captureOutput(func() {
		agt.HandleSlashCommands("/context")
	})
	assert.Contains(t, output, "0 words")
	assert.Contains(t, output, "0 messages")
}

func TestHandleSlashCommands_Context_WithHistory(t *testing.T) {
	agt := newTestAgent(t, nil)
	agt.SetChatHistory([]string{
		"User: Hello world",
		"Assistant: Hi there how are you",
		"User: Fine thanks",
	})

	output := captureOutput(func() {
		agt.HandleSlashCommands("/context")
	})
	assert.Contains(t, output, "words")
	assert.Contains(t, output, "3 messages")
}

// --- /compact tests ---

func TestHandleSlashCommands_Compact_EmptyHistory(t *testing.T) {
	agt := newTestAgent(t, nil)
	output := captureOutput(func() {
		agt.HandleSlashCommands("/compact")
	})
	assert.Contains(t, output, "No chat history to compact")
}

func TestHandleSlashCommands_Compact_Success(t *testing.T) {
	mockLLM := mocks.NewClient(t)
	agt := newTestAgent(t, mockLLM)

	agt.SetChatHistory([]string{
		"User: Hello",
		"Assistant: Hi there",
	})

	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(&llm.ChatResponse{
			Choices: []llm.Choice{
				{
					Message: llm.Message{
						Content: "User greeted, assistant responded",
					},
				},
			},
		}, nil).
		Maybe()

	output := captureOutput(func() {
		agt.HandleSlashCommands("/compact")
	})

	assert.Contains(t, output, "Compressed")

	history := agt.ChatHistory()
	require.Len(t, history, 1)
	assert.Contains(t, history[0], "User greeted")
}

func TestHandleSlashCommands_Compact_LLMError(t *testing.T) {
	mockLLM := mocks.NewClient(t)
	agt := newTestAgent(t, mockLLM)

	agt.SetChatHistory([]string{
		"User: Hello",
		"Assistant: Hi there",
	})

	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(nil, assert.AnError).
		Maybe()

	output := captureOutput(func() {
		agt.HandleSlashCommands("/compact")
	})
	assert.Contains(t, output, "Error compacting")

	history := agt.ChatHistory()
	require.Len(t, history, 2)
}

// --- Unknown command ---

func TestHandleSlashCommands_UnknownCommand(t *testing.T) {
	agt := newTestAgent(t, nil)
	output := captureOutput(func() {
		agt.HandleSlashCommands("/unknown")
	})
	assert.Contains(t, output, "Unknown command")
}

// --- AgenticLoop tests ---

func TestAgenticLoop_NoToolCalls(t *testing.T) {
	mockLLM := mocks.NewClient(t)
	agt := newTestAgent(t, mockLLM)

	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(&llm.ChatResponse{
			Choices: []llm.Choice{
				{
					FinishReason: "stop",
					Message: llm.Message{
						Content: "Hello! How can I help you?",
					},
				},
			},
		}, nil).
		Once()

	output := captureOutput(func() {
		agt.AgenticLoop("hi", nil)
	})
	assert.Contains(t, output, "Hello! How can I help you?")

	history := agt.ChatHistory()
	require.Len(t, history, 2)
	assert.Equal(t, "User: hi", history[0])
	assert.Contains(t, history[1], "Hello!")
}

func TestAgenticLoop_ToolCallThenResponse(t *testing.T) {
	mockLLM := mocks.NewClient(t)
	agt := newTestAgent(t, mockLLM)

	// First call: model wants to call a tool
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(&llm.ChatResponse{
			Choices: []llm.Choice{
				{
					FinishReason: "tool_calls",
					Message: llm.Message{
						Content: "",
						ToolCalls: []llm.ToolCall{
							{
								ID:   "call_1",
								Type: "function",
								Function: llm.ToolCallFunction{
									Name:      "read_file",
									Arguments: `{"file_name": "test.txt"}`,
								},
							},
						},
					},
				},
			},
		}, nil).
		Once()

	// Second call: after tool execution, model responds normally
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(&llm.ChatResponse{
			Choices: []llm.Choice{
				{
					FinishReason: "stop",
					Message: llm.Message{
						Content: "The file contains: hello world",
					},
				},
			},
		}, nil).
		Once()

	output := captureOutput(func() {
		agt.AgenticLoop("read test.txt", nil)
	})
	assert.Contains(t, output, "The file contains:")
}

func TestAgenticLoop_LLMError(t *testing.T) {
	mockLLM := mocks.NewClient(t)
	agt := newTestAgent(t, mockLLM)

	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(nil, assert.AnError).
		Once()

	output := captureOutput(func() {
		agt.AgenticLoop("hi", nil)
	})
	_ = output

	history := agt.ChatHistory()
	require.Len(t, history, 1)
	assert.Equal(t, "User: hi", history[0])
}

// --- Unit tests for helper functions ---

func TestConvertToolsToAPIFormat(t *testing.T) {
	tools := []llm.Tool{
		{
			ToolName:    "read_file",
			Description: "Reads a file",
			Params: llm.ToolParams{
				Type: "object",
				Properties: map[string]llm.Property{
					"file_name": {Type: "string", Description: "File to read"},
				},
				Required: []string{"file_name"},
			},
		},
	}

	result := convertToolsToAPIFormat(tools)
	require.Len(t, result, 1)
	assert.Equal(t, "function", result[0].Type)
	assert.Equal(t, "read_file", result[0].Function.Name)
}

func TestCreateMessagesFromHistory(t *testing.T) {
	agt := newTestAgent(t, nil)
	agt.SetChatHistory([]string{
		"User: Hello",
		"Assistant: World",
		"Tool result: something",
	})

	messages := agt.createMessagesFromHistory()
	require.Len(t, messages, 3)

	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "Hello", messages[0].Content)

	assert.Equal(t, "assistant", messages[1].Role)
	assert.Equal(t, "World", messages[1].Content)

	assert.Equal(t, "user", messages[2].Role)
	assert.Equal(t, "Tool result: something", messages[2].Content)
}

func TestPrintSeparator(t *testing.T) {
	output := captureOutput(func() {
		printSeparator()
	})
	assert.Contains(t, output, strings.Repeat("━", 60))
}
