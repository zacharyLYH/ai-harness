package main

import (
	"os"
	"strings"

	"ai-harness/agent"
	"ai-harness/common/logger"
	"ai-harness/common/tui"
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
		tui.PrintErr(loggerErr, "creating logger")
		os.Exit(1)
	}
	defer appLogger.Close()

	appLogger.SystemLog("Starting ai-harness application")

	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		tui.PrintErr(err, "loading .env file")
		os.Exit(1)
	}

	// Run linting on tools
	appLogger.SystemLog("Running tool linting...")
	if !tools.RunToolLinting(appLogger.UserPrint) {
		tui.Print("Tool linting failed. Please fix the errors above.")
		os.Exit(1)
	}
	appLogger.SystemLog("Tool linting passed!")

	// Load all tools from the tools directory
	toolManager := tools.NewDefaultToolManager()
	toolList, err := toolManager.LoadTools()
	if err != nil {
		tui.PrintErr(err, "loading tools")
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
	agt := agent.New(llmClient, toolManager, appLogger, loadedSkills)

	tui.Print("Welcome to ai-harness project — type your prompt or /help")
	tui.Sep()

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "  > ",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		tui.PrintErr(err, "initializing input")
		os.Exit(1)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				break
			}
			tui.PrintErr(err, "reading input")
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
	tui.Print("Bye")
}
