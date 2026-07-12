package interaction_test

import (
	"testing"
	"time"

	"ai-harness/integration"
	"ai-harness/llm"
	"ai-harness/llm/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var curlWebTool = llm.Tool{
	ToolName:    "curl_web",
	Description: "Fetches a webpage via curl and returns the core text content with HTML stripped out",
	PathToTool:  "tools/curl_web.py",
	Params: llm.ToolParams{
		Type: "object",
		Properties: map[string]llm.Property{
			"url": {Type: "string", Description: "The URL to fetch"},
		},
		Required: []string{"url"},
	},
}

var testTools = []llm.Tool{
	{
		ToolName:    "read_file",
		Description: "Reads a file",
		PathToTool:  "tools/read_file.py",
		Params: llm.ToolParams{
			Type: "object",
			Properties: map[string]llm.Property{
				"file_name": {Type: "string", Description: "File to read"},
			},
			Required: []string{"file_name"},
		},
	},
}

var writeTool = llm.Tool{
	ToolName:    "write_file",
	Description: "Writes content to a file",
	PathToTool:  "tools/write_file.py",
	Params: llm.ToolParams{
		Type: "object",
		Properties: map[string]llm.Property{
			"file_name": {Type: "string", Description: "File to write"},
			"content":   {Type: "string", Description: "Content to write"},
		},
		Required: []string{"file_name", "content"},
	},
}

// ---------------------------------------------------------------------------
// Core tool tests
// ---------------------------------------------------------------------------

func TestToolCallAndResponse(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("read the file"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "read_file", `{"file_name":"test.txt"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("The file says hello"), nil).Once()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, testTools)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("read test.txt")
	h.Expect("Allow?", 5*time.Second)
	h.Send("y")
	h.Expect("The file says hello", 10*time.Second)
}

func TestInitialPromptIncludesChecklistAndAllTools_SubagentsExcludeChecklist(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	toolList := append(append([]llm.Tool{}, testTools...), curlWebTool)
	var initialTools, subagentTools []string
	captureToolNames := func(definitions []llm.ToolDefinition) []string {
		names := make([]string, len(definitions))
		for i, definition := range definitions {
			names[i] = definition.Function.Name
		}
		return names
	}

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Run(func(_ []llm.Message, definitions []llm.ToolDefinition) {
			initialTools = captureToolNames(definitions)
		}).
		Return(&llm.ChatResponse{Choices: []llm.Choice{{
			FinishReason: "tool_calls",
			Message: llm.Message{ToolCalls: []llm.ToolCall{{
				ID: "checklist",
				Function: llm.ToolCallFunction{
					Name:      "create_checklist",
					Arguments: `{"items":[{"id":"one","description":"First step","seed_context":""},{"id":"two","description":"Second step","seed_context":""}]}`,
				},
			}}},
		}}}, nil).
		Once()
	mockLLM.EXPECT().
		Chat(mock.Anything, mock.Anything).
		Run(func(_ []llm.Message, definitions []llm.ToolDefinition) {
			subagentTools = captureToolNames(definitions)
		}).
		Return(integration.NewStopResponse("first step complete"), nil).
		Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("second step complete"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("Summary complete"), nil).Once()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, toolList)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("research this")
	h.Expect("Summary complete", 10*time.Second)

	assert.Equal(t, []string{"create_checklist", "read_file", "curl_web"}, initialTools)
	assert.Equal(t, []string{"read_file", "curl_web"}, subagentTools)
}

func TestToolConsentPrompt(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("read the file"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "read_file", `{"file_name":"test.txt"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("File read successfully"), nil).Once()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, testTools)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("read test.txt")
	h.Expect("Allow?", 5*time.Second)
	h.Send("y")
	h.Expect("File read successfully", 10*time.Second)
}

// ---------------------------------------------------------------------------
// Multi-tool consent
// ---------------------------------------------------------------------------

func TestMultipleToolCallsAllNeedConsent(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("read file and fetch url"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "read_file", `{"file_name":"a.txt"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_2", "curl_web", `{"url":"https://example.com"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("Done with both tools"), nil).Once()

	tools := append(testTools, curlWebTool)
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, tools)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("read and fetch")

	h.Expect("Allow?", 5*time.Second)
	h.Send("y")

	h.Expect("Allow?", 5*time.Second)
	h.Send("y")

	h.Expect("Done with both tools", 10*time.Second)
}

func TestRepeatedToolCallSkipsConsent(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("read the same file twice"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "read_file", `{"file_name":"a.txt"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_2", "read_file", `{"file_name":"a.txt"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("read_file complete"), nil).Once()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, testTools)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("read a.txt twice")

	// Only one consent prompt — second call should be auto-approved
	h.Expect("Allow?", 5*time.Second)
	h.Send("y")

	h.Expect("read_file complete", 10*time.Second)
}

func TestDifferentArgsNeedSeparateConsent(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("read two files"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "read_file", `{"file_name":"a.txt"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_2", "read_file", `{"file_name":"b.txt"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("both files read"), nil).Once()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, testTools)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("read a.txt and b.txt")

	// First consent
	h.Expect("Allow?", 5*time.Second)
	h.Send("y")

	// Second consent (different args → different allowlist key)
	h.Expect("Allow?", 5*time.Second)
	h.Send("y")

	h.Expect("both files read", 10*time.Second)
}

func TestApproveAllToolCalls(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("read two files"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "read_file", `{"file_name":"a.txt"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_2", "read_file", `{"file_name":"b.txt"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("both files read"), nil).Once()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, testTools)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("read a.txt and b.txt")
	h.Expect("Allow?", 5*time.Second)
	h.Send("a")
	h.Expect("both files read", 10*time.Second)
}

// ---------------------------------------------------------------------------
// Tool + curl_web
// ---------------------------------------------------------------------------

func TestCurlWebTool(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("fetch example.com"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "curl_web", `{"url":"https://example.com"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("Fetched example.com successfully"), nil).Once()

	tools := []llm.Tool{curlWebTool}
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, tools)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("fetch example.com")
	h.Expect("Allow?", 5*time.Second)
	h.Send("y")
	h.Expect("Fetched example.com successfully", 10*time.Second)
}

// ---------------------------------------------------------------------------
// New edge cases
// ---------------------------------------------------------------------------

func TestToolConsentShowsArgsInPrompt(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("read the file"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "read_file", `{"file_name":"secrets.txt"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("done"), nil).Once()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, testTools)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("read secrets.txt")
	h.Expect("read_file", 5*time.Second)
	h.Send("y")
	h.Expect("done", 10*time.Second)
}

func TestToolAfterSlashCommand(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("read file"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "read_file", `{"file_name":"test.txt"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("File contents"), nil).Once()

	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, testTools)

	h.Expect("ai-harness > ", 3*time.Second)

	// Slash command first — doesn't call LLM
	h.Send("/help")
	h.Expect("Available commands:", 3*time.Second)

	// Then a tool call
	h.Send("read test.txt")
	h.Expect("Allow?", 10*time.Second)
	h.Send("y")
	h.Expect("File contents", 10*time.Second)
}

func TestThreeToolsInSequence(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("read, write, fetch"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "read_file", `{"file_name":"a.txt"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_2", "write_file", `{"file_name":"b.txt","content":"hello"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_3", "curl_web", `{"url":"https://example.com"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("all three done"), nil).Once()

	tools := append(testTools, writeTool, curlWebTool)
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, tools)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("do all three")

	h.Expect("Allow?", 5*time.Second)
	h.Send("y")

	h.Expect("Allow?", 5*time.Second)
	h.Send("y")

	h.Expect("Allow?", 5*time.Second)
	h.Send("y")

	h.Expect("all three done", 10*time.Second)
}

func TestToolWithMultipleRequiredParams(t *testing.T) {
	h := integration.NewConsoleHarness(t)
	defer h.Close()

	mockLLM := mocks.NewClient(t)
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewChecklistBypassResponse("write a file"), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewToolCallResponse("call_1", "write_file", `{"file_name":"out.txt","content":"hello world"}`), nil).Once()
	mockLLM.EXPECT().Chat(mock.Anything, mock.Anything).Return(integration.NewStopResponse("file written"), nil).Once()

	tools := []llm.Tool{writeTool}
	agt := integration.NewTestAgent(t, mockLLM)
	integration.BootLoop(t, h, agt, tools)

	h.Expect("ai-harness > ", 3*time.Second)
	h.Send("write hello world to out.txt")
	h.Expect("Allow?", 10*time.Second)
	h.Send("y")
	h.Expect("file written", 10*time.Second)
}
