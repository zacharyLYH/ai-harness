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
	"ai-harness/skills"
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

// --- /skills tests ---

func TestHandleSlashCommands_Skills_NoSkills(t *testing.T) {
	agt := newTestAgent(t, nil)
	output := captureOutput(func() {
		agt.HandleSlashCommands("/skills")
	})
	assert.Contains(t, output, "No skills loaded")
}

func TestHandleSlashCommands_Skills_WithSkills(t *testing.T) {
	agt := newTestAgent(t, nil)
	agt.SetSkills([]skills.Skill{
		{Name: "write-a-poem", Description: "How to write a poem", Instructions: "Use old english"},
		{Name: "code-review", Description: "Review code", Instructions: "Check for bugs"},
	})

	output := captureOutput(func() {
		agt.HandleSlashCommands("/skills")
	})
	assert.Contains(t, output, "write-a-poem")
	assert.Contains(t, output, "How to write a poem")
	assert.Contains(t, output, "code-review")
	assert.Contains(t, output, "Review code")
	assert.Contains(t, output, "2")
}

// --- createMessagesFromHistory tests with skills ---

func TestCreateMessagesFromHistory_NoSkills(t *testing.T) {
	agt := newTestAgent(t, nil)
	agt.SetChatHistory([]string{
		"User: Hello",
		"Assistant: World",
	})

	messages := agt.createMessagesFromHistory()
	require.Len(t, messages, 2)
	// No system message since there are no skills
	for _, m := range messages {
		assert.NotEqual(t, "system", m.Role)
	}
}

func TestCreateMessagesFromHistory_SkillsInjectedOnce(t *testing.T) {
	agt := newTestAgent(t, nil)
	agt.SetSkills([]skills.Skill{
		{Name: "write-a-poem", Description: "How to write a poem", Instructions: "Use old english"},
	})
	agt.SetChatHistory([]string{
		"User: Hello",
		"Assistant: World",
	})

	messages := agt.createMessagesFromHistory()
	require.Len(t, messages, 3)

	// First message should be system with skills context
	assert.Equal(t, "system", messages[0].Role)
	content, ok := messages[0].Content.(string)
	require.True(t, ok)
	assert.Contains(t, content, "write-a-poem")
	assert.Contains(t, content, "How to write a poem")

	// Followed by user/assistant messages
	assert.Equal(t, "user", messages[1].Role)
	assert.Equal(t, "Hello", messages[1].Content)

	assert.Equal(t, "assistant", messages[2].Role)
	assert.Equal(t, "World", messages[2].Content)
}

func TestCreateMessagesFromHistory_SkillsNotReInjectedOnSecondTurn(t *testing.T) {
	agt := newTestAgent(t, nil)
	agt.SetSkills([]skills.Skill{
		{Name: "my-skill", Description: "A skill", Instructions: "Do the thing"},
	})
	// Simulate chat history from a first turn where skills were already injected
	agt.SetChatHistory([]string{
		"User: First message",
		"Assistant: First response",
	})

	// First call: skills should be present
	messages1 := agt.createMessagesFromHistory()
	require.Len(t, messages1, 3)
	assert.Equal(t, "system", messages1[0].Role)

	// Add another turn to the history
	agt.SetChatHistory(append(agt.ChatHistory(),
		"User: Second message",
		"Assistant: Second response",
	))

	// Second call: skills should NOT be re-injected (no duplicate system message)
	messages2 := agt.createMessagesFromHistory()
	// Still only one system message at the start
	require.Len(t, messages2, 5)
	assert.Equal(t, "system", messages2[0].Role)
	assert.Contains(t, messages2[0].Content.(string), "my-skill")

	assert.Equal(t, "user", messages2[1].Role)
	assert.Equal(t, "First message", messages2[1].Content)

	assert.Equal(t, "assistant", messages2[2].Role)

	assert.Equal(t, "user", messages2[3].Role)
	assert.Equal(t, "Second message", messages2[3].Content)

	assert.Equal(t, "assistant", messages2[4].Role)
}

func TestCreateMessagesFromHistory_MultipleSkills(t *testing.T) {
	agt := newTestAgent(t, nil)
	agt.SetSkills([]skills.Skill{
		{Name: "skill-one", Description: "First skill", Instructions: "Step 1"},
		{Name: "skill-two", Description: "Second skill", Instructions: "Step A"},
	})
	agt.SetChatHistory([]string{
		"User: Hi",
	})

	messages := agt.createMessagesFromHistory()
	require.Len(t, messages, 2)

	sysContent := messages[0].Content.(string)
	assert.Contains(t, sysContent, "skill-one")
	assert.Contains(t, sysContent, "First skill")
	assert.Contains(t, sysContent, "skill-two")
	assert.Contains(t, sysContent, "Second skill")
	assert.Contains(t, sysContent, "follow those instructions carefully")
}

// --- AgenticLoop with skills integration test ---

func TestAgenticLoop_WithSkills_InjectInPrompt(t *testing.T) {
	mockLLM := mocks.NewClient(t)
	agt := newTestAgent(t, mockLLM)
	agt.SetSkills([]skills.Skill{
		{Name: "write-a-poem", Description: "How to write a poem", Instructions: "Use old english\nBe humorous"},
	})

	var capturedMessages []llm.Message
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Run(func(messages []llm.Message, tools []llm.ToolDefinition) {
			capturedMessages = make([]llm.Message, len(messages))
			copy(capturedMessages, messages)
		}).
		Return(&llm.ChatResponse{
			Choices: []llm.Choice{
				{
					FinishReason: "stop",
					Message: llm.Message{
						Content: "Here's a poem for you!",
					},
				},
			},
		}, nil).
		Once()

	output := captureOutput(func() {
		agt.AgenticLoop("write a poem", nil)
	})
	assert.Contains(t, output, "Here's a poem for you!")

	// Verify the skill was included in the system message
	require.GreaterOrEqual(t, len(capturedMessages), 2)
	sysMsg := capturedMessages[0]
	assert.Equal(t, "system", sysMsg.Role)
	content, ok := sysMsg.Content.(string)
	require.True(t, ok)
	assert.Contains(t, content, "write-a-poem")
	assert.Contains(t, content, "How to write a poem")
	assert.Contains(t, content, "Use old english")
	assert.Contains(t, content, "Be humorous")
}

func TestAgenticLoop_WithSkills_OnlyOneSystemMessage(t *testing.T) {
	mockLLM := mocks.NewClient(t)
	agt := newTestAgent(t, mockLLM)
	agt.SetSkills([]skills.Skill{
		{Name: "test-skill", Description: "Test", Instructions: "Do test"},
	})

	var callCount int
	var firstCallMessages, secondCallMessages []llm.Message

	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Run(func(messages []llm.Message, tools []llm.ToolDefinition) {
			if callCount == 0 {
				firstCallMessages = make([]llm.Message, len(messages))
				copy(firstCallMessages, messages)
			} else {
				secondCallMessages = make([]llm.Message, len(messages))
				copy(secondCallMessages, messages)
			}
			callCount++
		}).
		Return(&llm.ChatResponse{
			Choices: []llm.Choice{
				{
					FinishReason: "stop",
					Message: llm.Message{
						Content: "response",
					},
				},
			},
		}, nil).
		Times(2)

	// First turn
	captureOutput(func() {
		agt.AgenticLoop("first prompt", nil)
	})

	// Second turn
	captureOutput(func() {
		agt.AgenticLoop("second prompt", nil)
	})

	// First call: should have system + user
	require.Len(t, firstCallMessages, 2)
	assert.Equal(t, "system", firstCallMessages[0].Role)
	assert.Equal(t, "user", firstCallMessages[1].Role)

	// Second call: should still only have ONE system message, plus 2 user + 1 assistant
	// The assistant response from the first turn is also in history
	require.Len(t, secondCallMessages, 4)
	assert.Equal(t, "system", secondCallMessages[0].Role)
	sysCount := 0
	for _, m := range secondCallMessages {
		if m.Role == "system" {
			sysCount++
		}
	}
	assert.Equal(t, 1, sysCount, "should have exactly one system message")
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

func TestPrintSeparator(t *testing.T) {
	output := captureOutput(func() {
		printSeparator()
	})
	assert.Contains(t, output, strings.Repeat("━", 60))
}

