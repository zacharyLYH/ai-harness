package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ai-harness/llm"
)

// AssetBase returns the directory that contains the app's bundled assets.
// When run from the repo it is the current directory; otherwise it falls back
// to the directory of the executable so an installed binary finds its files
// regardless of where it is launched from.
func AssetBase() string {
	if _, err := os.Stat("tools"); err == nil {
		return "."
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	return "."
}

var ChecklistToolDefinition = llm.ToolDefinition{
	Type: "function",
	Function: llm.ToolFunction{
		Name:        "create_checklist",
		Description: "Mandatory first step to plan the execution of the user's request. Create a checklist of subtasks. If the task is simple and doesn't require subtasks, return an empty array or an array with 1 item.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"items": map[string]interface{}{
					"type":        "array",
					"description": "List of tasks to complete",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id":           map[string]interface{}{"type": "string", "description": "Unique ID for the task"},
							"description":  map[string]interface{}{"type": "string", "description": "What to do"},
							"seed_context": map[string]interface{}{"type": "string", "description": "Starter code or context"},
						},
						"required": []string{"id", "description", "seed_context"},
					},
				},
			},
			"required": []string{"items"},
		},
	},
}

// LintRunner runs linting on tool files.
type LintRunner interface {
	RunLint() bool
}

// ToolLoader loads and parses tools from the filesystem.
type ToolLoader interface {
	LoadTools() ([]llm.Tool, error)
}

// ToolExecutor executes a tool given its name, arguments, and the tool list.
type ToolExecutor interface {
	Execute(toolName string, argumentsJSON string, tools []llm.Tool) string
}

// DefaultToolManager implements ToolLoader and ToolExecutor.
type DefaultToolManager struct{}

func NewDefaultToolManager() *DefaultToolManager {
	return &DefaultToolManager{}
}

// RunToolLinting runs the lint_tools.py script and returns true if linting passes.
func RunToolLinting(printer func(format string, args ...interface{})) bool {
	cmd := exec.Command("python3", filepath.Join(AssetBase(), "tools", "lint_tools.py"))
	output, err := cmd.CombinedOutput()

	if err != nil {
		if len(output) > 0 {
			printer("Linting output:\n%s", string(output))
		}
		return false
	}

	if len(output) > 0 {
		printer("Linting output:\n%s", string(output))
		return false
	}

	return true
}

// LoadTools reads all Python tool files in the tools directory and returns Tool objects.
func (m *DefaultToolManager) LoadTools() ([]llm.Tool, error) {
	toolsDir := filepath.Join(AssetBase(), "tools")
	var tools []llm.Tool

	files, err := os.ReadDir(toolsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read tools directory: %v", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()
		if !strings.HasSuffix(filename, ".py") || filename == "lint_tools.py" {
			continue
		}

		filePath := fmt.Sprintf("%s/%s", toolsDir, filename)
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		tool, err := parseToolMetadata(string(content), filePath)
		if err != nil {
			continue
		}

		tools = append(tools, tool)
	}

	if len(tools) == 0 {
		return nil, fmt.Errorf("no valid tools found in %s directory", toolsDir)
	}

	return tools, nil
}

// Execute runs a tool given its name, arguments JSON, and the list of loaded tools.
func (m *DefaultToolManager) Execute(toolName string, argumentsJSON string, tools []llm.Tool) string {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return fmt.Sprintf("Error parsing arguments: %v", err)
	}

	for _, tool := range tools {
		if tool.ToolName == toolName {
			cmdArgs := []string{tool.PathToTool}

			for _, paramName := range tool.Params.Required {
				if paramValue, ok := args[paramName]; ok {
					if strValue, ok := paramValue.(string); ok {
						cmdArgs = append(cmdArgs, strValue)
					} else {
						cmdArgs = append(cmdArgs, fmt.Sprintf("%v", paramValue))
					}
				}
			}

			for paramName, paramValue := range args {
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

// parseToolMetadata extracts tool information from the JSON metadata in Python files.
func parseToolMetadata(content string, filePath string) (llm.Tool, error) {
	lines := strings.Split(content, "\n")
	inMetadata := false
	metadataLines := []string{}

	for _, line := range lines {
		if strings.Contains(line, "\"\"\"") {
			if inMetadata {
				metadataLines = append(metadataLines, line)
				break
			} else {
				inMetadata = true
				metadataLines = append(metadataLines, line)
			}
		} else if inMetadata {
			metadataLines = append(metadataLines, line)
		}
	}

	if !inMetadata || len(metadataLines) == 0 {
		return llm.Tool{}, fmt.Errorf("no metadata found in %s", filePath)
	}

	metadataText := strings.Join(metadataLines, "\n")
	startIdx := strings.Index(metadataText, "{")
	endIdx := strings.LastIndex(metadataText, "}")

	if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
		return llm.Tool{}, fmt.Errorf("invalid metadata format in %s", filePath)
	}

	jsonText := metadataText[startIdx : endIdx+1]

	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(jsonText), &metadata); err != nil {
		return llm.Tool{}, fmt.Errorf("failed to parse metadata JSON in %s: %v", filePath, err)
	}

	toolName, ok := metadata["Name"].(string)
	if !ok {
		return llm.Tool{}, fmt.Errorf("missing or invalid 'Name' field in metadata of %s", filePath)
	}

	description, ok := metadata["Description"].(string)
	if !ok {
		return llm.Tool{}, fmt.Errorf("missing or invalid 'Description' field in metadata of %s", filePath)
	}

	paramsData, ok := metadata["Params"].(map[string]interface{})
	if !ok {
		return llm.Tool{}, fmt.Errorf("missing or invalid 'Params' field in metadata of %s", filePath)
	}

	toolParams, err := convertToToolParams(paramsData)
	if err != nil {
		return llm.Tool{}, fmt.Errorf("failed to convert params in %s: %v", filePath, err)
	}

	return llm.Tool{
		ToolName:    toolName,
		Description: description,
		PathToTool:  filePath,
		Params:      toolParams,
	}, nil
}

// convertToToolParams converts the JSON params structure to ToolParams.
func convertToToolParams(paramsData map[string]interface{}) (llm.ToolParams, error) {
	params := llm.ToolParams{
		Type:       "object",
		Properties: make(map[string]llm.Property),
		Required:   []string{},
	}

	propsData, ok := paramsData["Properties"].([]interface{})
	if !ok {
		return params, fmt.Errorf("missing or invalid 'Properties' field in params")
	}

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

			params.Properties[propName] = llm.Property{
				Type:        propType,
				Description: propDesc,
			}
		}
	}

	if reqData, ok := paramsData["Required"].([]interface{}); ok {
		for _, reqItem := range reqData {
			if reqStr, ok := reqItem.(string); ok {
				params.Required = append(params.Required, reqStr)
			}
		}
	}

	return params, nil
}

// GetToolNames returns a slice of tool names for logging.
func GetToolNames(tools []llm.Tool) []string {
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.ToolName
	}
	return names
}
