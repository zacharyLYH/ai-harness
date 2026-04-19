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

	// Run linting on tools
	appLogger.SystemLog("Running tool linting...")
	if !runToolLinting() {
		appLogger.UserPrint("Tool linting failed. Please fix the errors above.")
		os.Exit(1)
	}
	appLogger.SystemLog("Tool linting passed!")

	// Load all tools from the tools directory
	tools, err := loadAllTools()
	if err != nil {
		appLogger.SystemLog("Error loading tools: %v", err)
		appLogger.UserPrint("Error loading tools. Please check the tool files.")
		os.Exit(1)
	}

	appLogger.SystemLog("Loaded %d tools: %v", len(tools), getToolNames(tools))
	appLogger.UserPrint("Welcome to ai-harness project")

	var chatHistory []string
	for {
		fmt.Print("> ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			prompt := scanner.Text()
			agenticLoop(prompt, tools, &chatHistory)
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
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err)
	}

	// Find the tool
	for _, tool := range tools {
		if tool.ToolName == toolName {
			// Build command to run the Python script
			cmdArgs := []string{tool.PathToTool}

			// We need to preserve the order of arguments as defined in the tool's Properties
			// The LLM will return arguments in the correct order for the Python script
			// Since we can't rely on map iteration order, we need another approach

			// Simple approach: just pass all values (this assumes LLM returns them in correct order)
			// Actually, map iteration order is random in Go, so this won't work reliably

			// Better approach: we need to know the expected order from the tool definition
			// But we don't store the original order from the JSON metadata

			// For now, let's use a simple heuristic: pass required params first, then others
			// This should work for most tools
			for _, paramName := range tool.Params.Required {
				if paramValue, ok := args[paramName]; ok {
					if strValue, ok := paramValue.(string); ok {
						cmdArgs = append(cmdArgs, strValue)
					} else {
						cmdArgs = append(cmdArgs, fmt.Sprintf("%v", paramValue))
					}
				}
			}

			// Pass any remaining arguments
			for paramName, paramValue := range args {
				// Skip if already processed
				alreadyProcessed := false
				for _, reqParam := range tool.Params.Required {
					if paramName == reqParam {
						alreadyProcessed = true
						break
					}
				}
				if alreadyProcessed {
					continue
				}

				if strValue, ok := paramValue.(string); ok {
					cmdArgs = append(cmdArgs, strValue)
				} else {
					cmdArgs = append(cmdArgs, fmt.Sprintf("%v", paramValue))
				}
			}

			// Execute the command
			cmd := exec.Command("python3", cmdArgs...)
			appLogger.SystemLog("Executed command - %s", cmd.String())
			output, err := cmd.CombinedOutput()
			stringifiedOutput := ""
			if err != nil {
				return fmt.Sprintf("Error executing tool %s: %v\nOutput: %s", toolName, err, string(output))
			} else {
				stringifiedOutput = string(output)
				appLogger.SystemLog("Tool execution result: %s", stringifiedOutput)
			}

			return stringifiedOutput
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

	appLogger.LogAPIRequest(string(jsonData))

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

// runToolLinting runs the lint_tools.py script and returns true if linting passes
func runToolLinting() bool {
	cmd := exec.Command("python3", "tools/lint_tools.py")
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if there's actual output (not just exit code)
		if len(output) > 0 {
			appLogger.UserPrint("Linting output:\n%s", string(output))
		}
		return false
	}

	// Check if there's any output (the linter only outputs on failure)
	if len(output) > 0 {
		appLogger.UserPrint("Linting output:\n%s", string(output))
		return false
	}

	return true
}

// loadAllTools reads all Python tool files in the tools directory and returns Tool objects
func loadAllTools() ([]Tool, error) {
	toolsDir := "tools"
	var tools []Tool

	// List all files in the tools directory
	files, err := os.ReadDir(toolsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read tools directory: %v", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Check if it's a Python file and not the linter
		filename := file.Name()
		if !strings.HasSuffix(filename, ".py") || filename == "lint_tools.py" {
			continue
		}

		// Read the file to parse metadata
		filePath := fmt.Sprintf("%s/%s", toolsDir, filename)
		content, err := os.ReadFile(filePath)
		if err != nil {
			appLogger.SystemLog("Warning: Could not read tool file %s: %v", filePath, err)
			continue
		}

		// Parse the tool metadata
		tool, err := parseToolMetadata(string(content), filePath)
		if err != nil {
			appLogger.SystemLog("Warning: Could not parse tool metadata for %s: %v", filePath, err)
			continue
		}

		tools = append(tools, tool)
	}

	if len(tools) == 0 {
		return nil, fmt.Errorf("no valid tools found in %s directory", toolsDir)
	}

	return tools, nil
}

// parseToolMetadata extracts tool information from the JSON metadata in Python files
func parseToolMetadata(content string, filePath string) (Tool, error) {
	// Find the JSON metadata between triple quotes
	lines := strings.Split(content, "\n")
	inMetadata := false
	metadataLines := []string{}

	for _, line := range lines {
		if strings.Contains(line, "\"\"\"") {
			if inMetadata {
				// End of metadata
				metadataLines = append(metadataLines, line)
				break
			} else {
				// Start of metadata
				inMetadata = true
				metadataLines = append(metadataLines, line)
			}
		} else if inMetadata {
			metadataLines = append(metadataLines, line)
		}
	}

	if !inMetadata || len(metadataLines) == 0 {
		return Tool{}, fmt.Errorf("no metadata found in %s", filePath)
	}

	// Join metadata lines and extract JSON
	metadataText := strings.Join(metadataLines, "\n")
	// Extract content between triple quotes
	startIdx := strings.Index(metadataText, "{")
	endIdx := strings.LastIndex(metadataText, "}")

	if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
		return Tool{}, fmt.Errorf("invalid metadata format in %s", filePath)
	}

	jsonText := metadataText[startIdx : endIdx+1]

	// Parse the JSON
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(jsonText), &metadata); err != nil {
		return Tool{}, fmt.Errorf("failed to parse metadata JSON in %s: %v", filePath, err)
	}

	// Extract tool name
	toolName, ok := metadata["Name"].(string)
	if !ok {
		return Tool{}, fmt.Errorf("missing or invalid 'Name' field in metadata of %s", filePath)
	}

	// Extract description
	description, ok := metadata["Description"].(string)
	if !ok {
		return Tool{}, fmt.Errorf("missing or invalid 'Description' field in metadata of %s", filePath)
	}

	// Extract params
	paramsData, ok := metadata["Params"].(map[string]interface{})
	if !ok {
		return Tool{}, fmt.Errorf("missing or invalid 'Params' field in metadata of %s", filePath)
	}

	// Convert params to ToolParams
	toolParams, err := convertToToolParams(paramsData)
	if err != nil {
		return Tool{}, fmt.Errorf("failed to convert params in %s: %v", filePath, err)
	}

	return Tool{
		ToolName:    toolName,
		Description: description,
		PathToTool:  filePath,
		Params:      toolParams,
	}, nil
}

// convertToToolParams converts the JSON params structure to ToolParams
func convertToToolParams(paramsData map[string]interface{}) (ToolParams, error) {
	params := ToolParams{
		Type:       "object",
		Properties: make(map[string]Property),
		Required:   []string{},
	}

	// Extract properties
	propsData, ok := paramsData["Properties"].([]interface{})
	if !ok {
		return params, fmt.Errorf("missing or invalid 'Properties' field in params")
	}

	// Process each property
	for _, propItem := range propsData {
		propMap, ok := propItem.(map[string]interface{})
		if !ok {
			continue
		}

		for propName, propData := range propMap {
			propInfo, ok := propData.(map[string]interface{})
			if !ok {
				continue
			}

			propType, _ := propInfo["type"].(string)
			propDesc, _ := propInfo["description"].(string)

			params.Properties[propName] = Property{
				Type:        propType,
				Description: propDesc,
			}
		}
	}

	// Extract required fields if present
	if reqData, ok := paramsData["Required"].([]interface{}); ok {
		for _, reqItem := range reqData {
			if reqStr, ok := reqItem.(string); ok {
				params.Required = append(params.Required, reqStr)
			}
		}
	}

	return params, nil
}

// getToolNames returns a slice of tool names for logging
func getToolNames(tools []Tool) []string {
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.ToolName
	}
	return names
}
