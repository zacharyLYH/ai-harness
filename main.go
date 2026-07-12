package main

import (
	"bufio"
	"fmt"
	"os"

	"ai-harness/agent"
	"ai-harness/app"
	"ai-harness/common/logger"
	"ai-harness/common/tui"
	"ai-harness/llm"
	"ai-harness/setup"
	"ai-harness/skills"
	"ai-harness/tools"

	"github.com/joho/godotenv"
)

func main() {
	appLogger, loggerErr := logger.NewLogger()
	if loggerErr != nil {
		tui.PrintErr(loggerErr, "creating logger")
		os.Exit(1)
	}
	defer appLogger.Close()

	appLogger.SystemLog("Starting ai-harness application")

	if _, err := setup.Run(
		setup.RealFileIO{},
		func(prompt string) (string, error) {
			fmt.Print(prompt)
			return bufio.NewReader(os.Stdin).ReadString('\n')
		},
		func(apiKey string) error {
			return llm.NewOpenRouterClientWithKey(appLogger, apiKey).TestConnection()
		},
	); err != nil {
		tui.PrintErr(err, "setting up OPENROUTER_API_KEY")
		os.Exit(1)
	}

	err := godotenv.Load()
	if err != nil {
		tui.PrintErr(err, "loading .env file")
		os.Exit(1)
	}

	appLogger.SystemLog("Running tool linting...")
	if !tools.RunToolLinting(appLogger.UserPrint) {
		tui.Print("Tool linting failed. Please fix the errors above.")
		os.Exit(1)
	}
	appLogger.SystemLog("Tool linting passed!")

	toolManager := tools.NewDefaultToolManager()
	toolList, err := toolManager.LoadTools()
	if err != nil {
		tui.PrintErr(err, "loading tools")
		os.Exit(1)
	}
	appLogger.SystemLog("Loaded %d tools: %v", len(toolList), tools.GetToolNames(toolList))

	loadedSkills, err := skills.LoadAllSkills("skills/skills")
	if err != nil {
		appLogger.SystemLog("Warning: could not load skills: %v", err)
		loadedSkills = []skills.Skill{}
	}
	appLogger.SystemLog("Loaded %d skills", len(loadedSkills))

	llmClient := llm.NewOpenRouterClient(appLogger)
	agt := agent.New(llmClient, toolManager, appLogger, loadedSkills)

	tui.Print("Welcome to ai-harness project — type your prompt or /help")
	tui.Sep()

	app.RunInteractiveLoop(agt, toolList, "  > ", nil)

	tui.Print("Bye")
}
