package agent

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-harness/common/logger"
	"ai-harness/common/tui"
	"ai-harness/llm"
	"ai-harness/llm/mocks"
	"ai-harness/skills"
	"ai-harness/tools"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestAgent(t *testing.T, mockLLM *mocks.Client, skills []skills.Skill) *Agent {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "test.log")
	log, err := logger.NewLogger(logPath)
	require.NoError(t, err)
	t.Cleanup(func() { log.Close() })

	if mockLLM == nil {
		mockLLM = mocks.NewClient(t)
	}

	return New(mockLLM, tools.NewDefaultToolManager(), log, skills)
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
	agt := newTestAgent(t, nil, nil)
	output := captureOutput(func() {
		agt.HandleSlashCommands("/help")
	})
	assert.Contains(t, output, "Available commands:")
	assert.Contains(t, output, "/context")
	assert.Contains(t, output, "/compact")
	assert.Contains(t, output, "/help")
}

func TestHandleSlashCommands_EmptyString(t *testing.T) {
	agt := newTestAgent(t, nil, nil)
	output := captureOutput(func() {
		agt.HandleSlashCommands("")
	})
	assert.Empty(t, output)
}

// --- /context tests ---

func TestHandleSlashCommands_Context_Empty(t *testing.T) {
	agt := newTestAgent(t, nil, nil)
	output := captureOutput(func() {
		agt.HandleSlashCommands("/context")
	})
	assert.Contains(t, output, "0 words")
	assert.Contains(t, output, "0 messages")
}

// --- /compact tests ---

func TestHandleSlashCommands_Compact_EmptyHistory(t *testing.T) {
	agt := newTestAgent(t, nil, nil)
	output := captureOutput(func() {
		agt.HandleSlashCommands("/compact")
	})
	assert.Contains(t, output, "No chat history to compact")
}

// --- Unknown command ---

func TestHandleSlashCommands_UnknownCommand(t *testing.T) {
	agt := newTestAgent(t, nil, nil)
	output := captureOutput(func() {
		agt.HandleSlashCommands("/unknown")
	})
	assert.Contains(t, output, "Unknown command")
}

// --- AgenticLoop tests ---

func TestAgenticLoop_NoToolCalls(t *testing.T) {
	mockLLM := mocks.NewClient(t)
	agt := newTestAgent(t, mockLLM, nil)
	agt.hasChecklist = true

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

	history := agt.chatHistory
	require.Len(t, history, 2)
	assert.Equal(t, "User: hi", history[0])
	assert.Contains(t, history[1], "Hello!")
}

func TestAgenticLoop_InitialToolsIncludeChecklistAndSearch(t *testing.T) {
	mockLLM := mocks.NewClient(t)
	agt := newTestAgent(t, mockLLM, nil)
	searchTool := llm.Tool{ToolName: "duckduckgo_search", Description: "Searches the web"}

	var toolNames []string
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Run(func(_ []llm.Message, definitions []llm.ToolDefinition) {
			for _, definition := range definitions {
				toolNames = append(toolNames, definition.Function.Name)
			}
		}).
		Return(&llm.ChatResponse{Choices: []llm.Choice{{
			FinishReason: "stop",
			Message:      llm.Message{Content: "Done"},
		}}}, nil).
		Once()

	captureOutput(func() {
		agt.AgenticLoop("search the web", []llm.Tool{searchTool})
	})

	assert.Equal(t, []string{"create_checklist", "duckduckgo_search"}, toolNames)
}

func TestAgenticLoop_ToolCallThenResponse(t *testing.T) {
	mockLLM := mocks.NewClient(t)
	agt := newTestAgent(t, mockLLM, nil)
	agt.hasChecklist = true

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
	agt := newTestAgent(t, mockLLM, nil)
	agt.hasChecklist = true

	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(nil, assert.AnError).
		Once()

	output := captureOutput(func() {
		agt.AgenticLoop("hi", nil)
	})
	_ = output

	history := agt.chatHistory
	require.Len(t, history, 1)
	assert.Equal(t, "User: hi", history[0])
}

// --- /skills tests ---

func TestHandleSlashCommands_Skills_NoSkills(t *testing.T) {
	agt := newTestAgent(t, nil, nil)
	output := captureOutput(func() {
		agt.HandleSlashCommands("/skills")
	})
	assert.Contains(t, output, "No skills loaded")
}

// --- AgenticLoop with skills integration test ---

func TestAgenticLoop_WithSkills_InjectInPrompt(t *testing.T) {
	mockLLM := mocks.NewClient(t)
	agt := newTestAgent(t, mockLLM, []skills.Skill{
		{Name: "write-a-poem", Description: "How to write a poem", Instructions: "Use old english\nBe humorous"},
	})
	agt.hasChecklist = true

	var capturedMessages []llm.Message
	var capturedTools []llm.ToolDefinition
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Run(func(messages []llm.Message, tools []llm.ToolDefinition) {
			capturedMessages = make([]llm.Message, len(messages))
			copy(capturedMessages, messages)
			capturedTools = make([]llm.ToolDefinition, len(tools))
			copy(capturedTools, tools)
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

	// Verify the skill was included in the system message (name + description only)
	require.GreaterOrEqual(t, len(capturedMessages), 2)
	sysMsg := capturedMessages[0]
	assert.Equal(t, "system", sysMsg.Role)
	content, ok := sysMsg.Content.(string)
	require.True(t, ok)
	assert.Contains(t, content, "write-a-poem")
	assert.Contains(t, content, "How to write a poem")
	// Instructions should NOT be in the system prompt anymore
	assert.NotContains(t, content, "Use old english")
	assert.NotContains(t, content, "Be humorous")

	// Verify the skill was registered as a tool definition
	found := false
	for _, td := range capturedTools {
		if td.Function.Name == "write-a-poem" {
			found = true
			assert.Equal(t, "How to write a poem", td.Function.Description)
			break
		}
	}
	assert.True(t, found, "skill should be registered as a tool definition")
}

func TestAgenticLoop_WithSkills_OnlyOneSystemMessage(t *testing.T) {
	mockLLM := mocks.NewClient(t)
	agt := newTestAgent(t, mockLLM, nil)
	agt.hasChecklist = true

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
		tui.Sep()
	})
	assert.Contains(t, output, strings.Repeat("━", 60))
}

// --- Subagent tests ---

func TestSubagent_ReturnsResult(t *testing.T) {
	mockLLM := mocks.NewClient(t)
	logPath := filepath.Join(t.TempDir(), "test.log")
	log, err := logger.NewLogger(logPath)
	require.NoError(t, err)
	t.Cleanup(func() { log.Close() })

	allowlist := make(map[string]interface{})
	sub := &Agent{
		ID:            "sub-task-1",
		isSubagent:    true,
		printPrefix:   "  [sub-task-1] ",
		llmClient:     mockLLM,
		toolManager:   tools.NewDefaultToolManager(),
		logger:        log,
		chatHistory:   make([]string, 0),
		toolAllowlist: &allowlist,
		skills:        make([]skills.Skill, 0),
	}

	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(&llm.ChatResponse{
			Choices: []llm.Choice{
				{
					FinishReason: "stop",
					Message: llm.Message{
						Content: "I completed the task successfully",
					},
				},
			},
		}, nil).
		Once()

	var result string
	captureOutput(func() {
		result = sub.AgenticLoop("create a file", nil)
	})
	assert.Equal(t, "I completed the task successfully", result)
}

func TestAgenticLoop_ChecklistFlow(t *testing.T) {
	mockLLM := mocks.NewClient(t)
	agt := newTestAgent(t, mockLLM, nil)

	checklistArgs := `{"items": [{"id": "step-1", "description": "Create the CSV", "seed_context": "header: name,age"}, {"id": "step-2", "description": "Write the report", "seed_context": "summarize the data"}]}`

	// First call: parent agent gets checklist response
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(&llm.ChatResponse{
			Choices: []llm.Choice{
				{
					FinishReason: "tool_calls",
					Message: llm.Message{
						ToolCalls: []llm.ToolCall{
							{
								ID: "call-1",
								Function: llm.ToolCallFunction{
									Name:      "create_checklist",
									Arguments: checklistArgs,
								},
							},
						},
					},
				},
			},
		}, nil).
		Once()

	// Second call: subagent 1 completes
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(&llm.ChatResponse{
			Choices: []llm.Choice{
				{
					FinishReason: "stop",
					Message:      llm.Message{Content: "CSV created with headers name,age"},
				},
			},
		}, nil).
		Once()

	// Third call: subagent 2 completes
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(&llm.ChatResponse{
			Choices: []llm.Choice{
				{
					FinishReason: "stop",
					Message:      llm.Message{Content: "Report written summarizing 10 records"},
				},
			},
		}, nil).
		Once()

	// Fourth call: synthesis
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Return(&llm.ChatResponse{
			Choices: []llm.Choice{
				{
					FinishReason: "stop",
					Message:      llm.Message{Content: "All tasks completed: CSV created and report written."},
				},
			},
		}, nil).
		Once()

	var result string
	output := captureOutput(func() {
		result = agt.AgenticLoop("create csv and report", nil)
	})

	assert.Contains(t, output, "Checklist detected")
	assert.Contains(t, output, "Spawning subagent")
	assert.Equal(t, "All tasks completed: CSV created and report written.", result)
}

func TestSharedToolAllowlist(t *testing.T) {
	allowlist := make(map[string]interface{})
	allowlist["bash"+"cmd=ls"] = nil

	logPath := filepath.Join(t.TempDir(), "test.log")
	log, err := logger.NewLogger(logPath)
	require.NoError(t, err)
	t.Cleanup(func() { log.Close() })

	parent := &Agent{
		ID:            "root",
		toolAllowlist: &allowlist,
		logger:        log,
	}
	sub := &Agent{
		ID:            "sub-1",
		isSubagent:    true,
		toolAllowlist: parent.toolAllowlist, // shared
		logger:        log,
	}

	// Both should see the same allowlist
	assert.Equal(t, len(*parent.toolAllowlist), len(*sub.toolAllowlist))

	// Add through sub, parent should see it
	(*sub.toolAllowlist)["read_file"+"f=test.txt"] = nil
	assert.Contains(t, *parent.toolAllowlist, "read_file"+"f=test.txt")
}
