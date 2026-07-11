package main

import (
	"fmt"
	"os"
	"strings"

	"ai-harness/agent"
	"ai-harness/common/logger"
	"ai-harness/llm"
	"ai-harness/skills"
	"ai-harness/tools"

	"github.com/chzyer/readline"
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

	// Load all skills from the skills/skills directory
	loadedSkills, err := skills.LoadAllSkills("skills/skills")
	if err != nil {
		appLogger.SystemLog("Warning: could not load skills: %v", err)
		loadedSkills = []skills.Skill{}
	}
	appLogger.SystemLog("Loaded %d skills", len(loadedSkills))

	// Create LLM client
	llmClient := llm.NewOpenRouterClient(appLogger)

	// Create agent
	agt := agent.New(llmClient, toolManager, appLogger)
	agt.SetSkills(loadedSkills)

	fmt.Println("Welcome to ai-harness project — type your prompt or /help")
	printSeparator()

	rl, err := readline.New("  > ")
	if err != nil {
		fmt.Println("Error initializing input:", err)
		os.Exit(1)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				break
			}
			fmt.Println("Error reading input:", err)
			os.Exit(1)
		}

		prompt := strings.TrimSpace(line)
		if prompt == "" {
			continue
		}

		if strings.HasPrefix(prompt, "/") {
			agt.HandleSlashCommands(prompt)
			continue
		}

		agt.AgenticLoop(prompt, toolList)
	}
	fmt.Println("bye")
}

func printSeparator() {
	fmt.Println(strings.Repeat("━", 60))
}
