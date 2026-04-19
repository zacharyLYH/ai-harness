package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"ai-harness/common/logger"

	"github.com/joho/godotenv"
)

// Global logger instance
var appLogger *logger.Logger

// Tool definition for our system
type Tool struct {
	ToolName    string
	Description string
	PathToTool  string
	Params      ToolParams
}

type ToolParams struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// Request structures for OpenRouter API
type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ChatRequest struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	MaxTokens   int              `json:"max_tokens"`
	Temperature float64          `json:"temperature"`
}

// Response structures
type ChatResponse struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Model   string   `json:"model"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
	Index        int     `json:"index"`
}

type Usage struct {
	CompletionTokens int `json:"completion_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func main() {
	// Initialize logger
	var loggerErr error
	appLogger, loggerErr = logger.NewLogger()
	if loggerErr != nil {
		// Can't use logger if it failed, so use standard error
		fmt.Println("Error creating logger:", loggerErr)
		os.Exit(1)
	}
	defer appLogger.Close()

	appLogger.SystemLog("Starting ai-harness application")

	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		appLogger.SystemLog("Error loading .env file: %v", err)
		appLogger.UserPrint("Error loading .env file. Please check your configuration.")
		os.Exit(1)
	}

	appLogger.UserPrint("Welcome to ai-harness project")
	listFileTool := Tool{
		ToolName:    "ListFile",
		Description: "Lists files in the LLM's dedicated directory (llm_directory) using the ls command.",
		PathToTool:  "tools/list_file.py",
		Params: ToolParams{
			Type: "object",
			Properties: map[string]Property{
				"file_name": {
					Type:        "string",
					Description: "The name of the file to look for (can be pattern or wildcard). If not provided, lists all files.",
				},
			},
			Required: []string{},
		},
	}
	var chatHistory []string
	for {
		fmt.Print("> ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			prompt := scanner.Text()
			agenticLoop(prompt, []Tool{listFileTool}, &chatHistory)
		}
	}
}

func agenticLoop(prompt string, tools []Tool, chatHistory *[]string) {
	// Log user prompt
	appLogger.SystemLog("User prompt: %s", prompt)

	// Add user prompt to chat history
	*chatHistory = append((*chatHistory), fmt.Sprintf("User: %s", prompt))

	// Convert our internal tools to OpenRouter tool definitions
	apiTools := convertToolsToAPIFormat(tools)

	// Create initial messages from chat history
	messages := createMessagesFromHistory(*chatHistory)

	// Infinite loop until there are no tool calls
	for {
		// Send request
		response, err := callLLM(messages, apiTools)
		if err != nil {
			appLogger.SystemLog("Error calling LLM: %v", err)
			return
		}

		if len(response.Choices) == 0 {
			appLogger.SystemLog("No response from LLM")
			return
		}

		assistantMessage := response.Choices[0].Message
		assistantContent := ""

		// Convert assistant content to string if it's not already
		switch v := assistantMessage.Content.(type) {
		case string:
			assistantContent = v
		case nil:
			assistantContent = ""
		default:
			assistantContent = fmt.Sprintf("%v", v)
		}

		// Add assistant response to chat history
		*chatHistory = append((*chatHistory), fmt.Sprintf("Assistant: %s", assistantContent))

		appLogger.SystemLog("Finish reason: %s", response.Choices[0].FinishReason)

		// If the model wants to call a tool
		if response.Choices[0].FinishReason == "tool_calls" && len(assistantMessage.ToolCalls) > 0 {
			toolCall := assistantMessage.ToolCalls[0]

			// Execute the tool
			toolResult := executeTool(toolCall.Function.Name, toolCall.Function.Arguments, tools)

			// Add tool execution to chat history
			*chatHistory = append((*chatHistory), fmt.Sprintf("Tool %s executed: %s", toolCall.Function.Name, toolResult))

			// Append assistant message and tool result to messages
			messages = append(messages, assistantMessage)
			messages = append(messages, Message{
				Role:       "tool",
				Content:    toolResult,
				ToolCallID: toolCall.ID,
			})

			// Continue the loop to process tool result
			continue
		} else {
			// No tool calls, display the response and break the loop
			appLogger.UserPrint("Assistant Response:\n%s", assistantContent)

			// Break the infinite loop since there are no tool calls
			break
		}
	}
}

func createMessagesFromHistory(chatHistory []string) []Message {
	// Convert chat history to messages
	messages := []Message{}

	// Simple conversion: alternate between user and assistant messages
	for _, entry := range chatHistory {
		if strings.HasPrefix(entry, "User: ") {
			messages = append(messages, Message{
				Role:    "user",
				Content: strings.TrimPrefix(entry, "User: "),
			})
		} else if strings.HasPrefix(entry, "Assistant: ") {
			messages = append(messages, Message{
				Role:    "assistant",
				Content: strings.TrimPrefix(entry, "Assistant: "),
			})
		} else {
			// Handle tool messages or other formats
			messages = append(messages, Message{
				Role:    "user", // Default to user
				Content: entry,
			})
		}
	}

	return messages
}

func convertToolsToAPIFormat(tools []Tool) []ToolDefinition {
	apiTools := make([]ToolDefinition, len(tools))
	for i, tool := range tools {
		apiTools[i] = ToolDefinition{
			Type: "function",
			Function: ToolFunction{
				Name:        tool.ToolName,
				Description: tool.Description,
				Parameters:  tool.Params,
			},
		}
	}
	return apiTools
}

func executeTool(toolName string, arguments string, tools []Tool) string {
	// Parse arguments
	var args map[string]string
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err)
	}

	// Find the tool
	for _, tool := range tools {
		if tool.ToolName == toolName {
			// Get file_name argument (if provided)
			fileName := args["file_name"]

			// Hardcode directory to llm_directory
			directory := "llm_directory"

			// Build command to run the Python script
			cmdArgs := []string{tool.PathToTool, directory}
			if fileName != "" {
				cmdArgs = append(cmdArgs, fileName)
			}

			// Execute the command
			cmd := exec.Command("python3", cmdArgs...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Sprintf("Error executing tool %s: %v\nOutput: %s", toolName, err, string(output))
			}

			return string(output)
		}
	}

	return fmt.Sprintf("Tool %s not found", toolName)
}

func callLLM(messages []Message, tools []ToolDefinition) (*ChatResponse, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	model := "openrouter/free"
	temperature := 0.7
	maxToken := 1000

	requestBody := ChatRequest{
		Model:       model,
		Messages:    messages,
		Tools:       tools,
		MaxTokens:   maxToken,
		Temperature: temperature,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	url := "https://openrouter.ai/api/v1/chat/completions"

	// Exponential backoff retry logic
	maxRetries := 3
	var lastErr error
	var response *ChatResponse

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay: 1, 2, 4 seconds
			delay := 1 << (attempt - 1) // 1, 2, 4 seconds
			appLogger.SystemLog("Retry attempt %d/%d after %d seconds", attempt, maxRetries, delay)
			time.Sleep(time.Duration(delay) * time.Second)
		}

		req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
		if err != nil {
			lastErr = err
			appLogger.SystemLog("Failed to create request: %v", err)
			continue
		}

		authorizationHeader := fmt.Sprintf("Bearer %s", apiKey)
		req.Header.Add("Authorization", authorizationHeader)
		req.Header.Add("Content-Type", "application/json")

		appLogger.SystemLog("Sending API request (attempt %d/%d)", attempt+1, maxRetries+1)

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			appLogger.SystemLog("HTTP request failed: %v", err)
			continue
		}

		body, err := io.ReadAll(res.Body)
		res.Body.Close()

		if err != nil {
			lastErr = err
			appLogger.SystemLog("Failed to read response body: %v", err)
			continue
		}

		// Parse the response
		var chatResp ChatResponse
		if err := json.Unmarshal(body, &chatResp); err != nil {
			lastErr = err
			appLogger.SystemLog("Failed to parse JSON response: %v", err)
			continue
		}

		// Log the entire response
		appLogger.LogAPIResponse(chatResp)

		// Success - return the response
		response = &chatResp
		break
	}

	if response == nil {
		appLogger.SystemLog("All %d retry attempts failed", maxRetries)
		return nil, fmt.Errorf("LLM API failed after %d retries: %v", maxRetries, lastErr)
	}

	return response, nil
}
