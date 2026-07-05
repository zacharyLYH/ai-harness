package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"ai-harness/common/logger"
)

// Client defines the interface for LLM interactions.
// This can be mocked for testing.
type Client interface {
	Chat(messages []Message, tools []ToolDefinition) (*ChatResponse, error)
}

// OpenRouterClient implements Client using the OpenRouter API.
type OpenRouterClient struct {
	apiKey string
	logger *logger.Logger
}

// NewOpenRouterClient creates a new OpenRouter client.
func NewOpenRouterClient(logger *logger.Logger) *OpenRouterClient {
	return &OpenRouterClient{
		apiKey: os.Getenv("OPENROUTER_API_KEY"),
		logger: logger,
	}
}

// Chat sends a chat completion request to OpenRouter with exponential backoff retry.
func (c *OpenRouterClient) Chat(messages []Message, tools []ToolDefinition) (*ChatResponse, error) {
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

	c.logger.LogAPIRequest(string(jsonData))

	url := "https://openrouter.ai/api/v1/chat/completions"

	// Exponential backoff retry logic
	maxRetries := 3
	var lastErr error
	var response *ChatResponse

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay: 1, 2, 4 seconds
			delay := 1 << (attempt - 1) // 1, 2, 4 seconds
			c.logger.SystemLog("Retry attempt %d/%d after %d seconds", attempt, maxRetries, delay)
			time.Sleep(time.Duration(delay) * time.Second)
		}

		req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
		if err != nil {
			lastErr = err
			c.logger.SystemLog("Failed to create request: %v", err)
			continue
		}

		authorizationHeader := fmt.Sprintf("Bearer %s", c.apiKey)
		req.Header.Add("Authorization", authorizationHeader)
		req.Header.Add("Content-Type", "application/json")

		c.logger.SystemLog("Sending API request (attempt %d/%d)", attempt+1, maxRetries+1)

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			c.logger.SystemLog("HTTP request failed: %v", err)
			continue
		}

		body, err := io.ReadAll(res.Body)
		res.Body.Close()

		if err != nil {
			lastErr = err
			c.logger.SystemLog("Failed to read response body: %v", err)
			continue
		}

		// Parse the response
		var chatResp ChatResponse
		if err := json.Unmarshal(body, &chatResp); err != nil {
			lastErr = err
			c.logger.SystemLog("Failed to parse JSON response: %v", err)
			continue
		}

		// Log the entire response
		c.logger.LogAPIResponse(chatResp)

		// Success - return the response
		response = &chatResp
		break
	}

	if response == nil {
		c.logger.SystemLog("All %d retry attempts failed", maxRetries)
		return nil, fmt.Errorf("LLM API failed after %d retries: %v", maxRetries, lastErr)
	}

	return response, nil
}
