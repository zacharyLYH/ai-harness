package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Tool struct {
	ToolName    string
	Description string
	PathToTool  string
}

type Message struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

type LLMRequest struct {
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Model       string    `json:"model"`
	Temperature float64   `json:"temperature"`
}

type Usage struct {
	CompletionTokens int `json:"completion_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Choice struct {
	FinishReason string  `json:"finish_reason"`
	Index        int     `json:"index"`
	Message      Message `json:"message"`
}

type LLMResponse struct {
	Choices           []Choice `json:"choices"`
	Created           int64    `json:"created"`
	ID                string   `json:"id"`
	Model             string   `json:"model"`
	Object            string   `json:"object"`
	SystemFingerprint string   `json:"system_fingerprint"`
	Usage             Usage    `json:"usage"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	fmt.Println("Welcome to ai-harness project")
	listFileTool := Tool{
		ToolName: "ListFile",
		Description: `
		Lists files in a specified directory using the ls command.
		Args:
			file_name: The name of the file to look for (can be pattern or wildcard)
			directory: The directory path to search
		Returns:
			String containing the output of the ls command
		Raises:
			FileNotFoundError: If the directory doesn't exist
			subprocess.CalledProcessError: If the ls command fails
		`,
	}
	for {
		fmt.Print("> ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			prompt := scanner.Text()
			agenticLoop(prompt, []Tool{listFileTool})
		}
	}
}

func agenticLoop(prompt string, tools []Tool) {

	llmResponse, callLlMErr := callLLM(prompt, tools)
	if callLlMErr != nil {
		fmt.Println(callLlMErr.Error())
		return
	}

	if len(llmResponse.Choices) > 0 {
		fmt.Println("Response:", llmResponse.Choices[0].Message.Content)
	}

	// TODO
	//read response
	//see if there are any tool calls
	//execute tools

	fmt.Printf("Model: %s, ID: %s\n", llmResponse.Model, llmResponse.ID)
	fmt.Printf("Tokens used: %d prompt + %d completion = %d total\n",
		llmResponse.Usage.PromptTokens,
		llmResponse.Usage.CompletionTokens,
		llmResponse.Usage.TotalTokens)
}

func callLLM(prompt string, tools []Tool) (*LLMResponse, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	model := "openrouter/free"
	temperature := 0.7
	maxToken := 1000

	toolText := "List of tools include\n"
	/*
		{
			name: toolname,
			description: description,
		}
	*/
	for _, t := range tools {
		toolText += "{ name: "
		toolText += t.ToolName
		toolText += ", description: "
		toolText += t.Description
		toolText += "},\n"
	}

	// TEST
	toolText += `Return the tool to call like
	{
		Name: "Tool name",
		Params: ["param1","param2"]
	}
	`

	requestBody := LLMRequest{
		Messages: []Message{
			{
				Content: "You are a helpful assistant.",
				Role:    "system",
			},
			{
				Content: prompt + "\n" + toolText,
				Role:    "user",
			},
		},
		MaxTokens:   maxToken,
		Model:       model,
		Temperature: temperature,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	url := "https://openrouter.ai/api/v1/chat/completions"
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	authorizationHeader := fmt.Sprintf("Bearer %s", apiKey)
	req.Header.Add("Authorization", authorizationHeader)
	req.Header.Add("Content-Type", "application/json")

	fmt.Println("PAYLOAD: ", string(jsonData))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	// Parse the response into the typed structure
	var response LLMResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// Whats the weather today
