package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"ai-harness/agent"
	"ai-harness/common/logger"
	"ai-harness/llm"
	"ai-harness/tools"

	"github.com/joho/godotenv"
)

func main() {
	// Initialize logger
	appLogger, loggerErr := logger.NewLogger()
	if loggerErr != nil {
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
	if !tools.RunToolLinting(appLogger.UserPrint) {
		appLogger.UserPrint("Tool linting failed. Please fix the errors above.")
		os.Exit(1)
	}
	appLogger.SystemLog("Tool linting passed!")

	// Load all tools from the tools directory
	toolManager := tools.NewDefaultToolManager()
	toolList, err := toolManager.LoadTools()
	if err != nil {
		appLogger.SystemLog("Error loading tools: %v", err)
		appLogger.UserPrint("Error loading tools. Please check the tool files.")
		os.Exit(1)
	}

	appLogger.SystemLog("Loaded %d tools: %v", len(toolList), tools.GetToolNames(toolList))

	// Create LLM client
	llmClient := llm.NewOpenRouterClient(appLogger)

	// Create agent
	agt := agent.New(llmClient, toolManager, appLogger)

	fmt.Println("Welcome to ai-harness project — type your prompt or /help")
	printSeparator()

	for {
		fmt.Print("\n  > ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			prompt := strings.TrimSpace(scanner.Text())
			if prompt == "" {
				continue
			}

			if strings.HasPrefix(prompt, "/") {
				agt.HandleSlashCommands(prompt)
				continue
			}

			agt.AgenticLoop(prompt, toolList)
		}
	}
}

func printSeparator() {
	fmt.Println(strings.Repeat("━", 60))
}
