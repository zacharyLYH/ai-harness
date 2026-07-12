package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Logger handles both file logging and console output
type Logger struct {
	fileLogger *log.Logger
	logFile    *os.File
	rawLogger  *log.Logger
	rawFile    *os.File

	scope string
	depth int
}

// NewLogger creates a new logger instance
func NewLogger(filePath ...string) (*Logger, error) {
	path := "common/logger/log.txt"
	if len(filePath) > 0 {
		path = filePath[0]
	} else if _, err := os.Stat("common/logger"); err != nil {
		// Not launched from the repo root (e.g. installed binary run from
		// elsewhere): use a CWD-independent cache location instead.
		if cacheDir, cerr := os.UserCacheDir(); cerr == nil {
			path = filepath.Join(cacheDir, "ai-harness", "log.txt")
		}
	}
	rawPath := strings.Replace(path, ".txt", "_raw.txt", 1)
	if rawPath == path {
		rawPath = path + ".raw"
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	// os.O_TRUNC wipes the file if it exists.
	// os.O_CREATE creates it if it doesn't.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}

	rawFile, err := os.OpenFile(rawPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		file.Close()
		return nil, err
	}

	// Create a logger that writes to the file
	fileLogger := log.New(file, "", 0)
	rawLogger := log.New(rawFile, "", 0)

	return &Logger{
		fileLogger: fileLogger,
		logFile:    file,
		rawLogger:  rawLogger,
		rawFile:    rawFile,
		scope:      "root",
		depth:      0,
	}, nil
}

// Close closes the log files
func (l *Logger) Close() error {
	var err error
	if l.logFile != nil {
		if e := l.logFile.Close(); e != nil {
			err = e
		}
	}
	if l.rawFile != nil {
		if e := l.rawFile.Close(); e != nil {
			err = e
		}
	}
	return err
}

// WithScope returns a new logger instance with the given scope and depth
func (l *Logger) WithScope(scope string, depth int) *Logger {
	return &Logger{
		fileLogger: l.fileLogger,
		logFile:    l.logFile,
		rawLogger:  l.rawLogger,
		rawFile:    l.rawFile,
		scope:      scope,
		depth:      depth,
	}
}

// prefix returns the visual prefix for the current scope and depth
func (l *Logger) prefix() string {
	indent := strings.Repeat("    ", l.depth)
	if l.scope == "root" {
		return fmt.Sprintf("%s[root] ", indent)
	}
	return fmt.Sprintf("%s[%s] ", indent, l.scope)
}

// Log writes a message to the log file with timestamp and scope prefix
func (l *Logger) Log(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Add prefix to every line if message contains newlines
	lines := strings.Split(message, "\n")
	for _, line := range lines {
		logEntry := fmt.Sprintf("[%s] %s%s", timestamp, l.prefix(), line)
		l.fileLogger.Println(logEntry)
	}
}

// SystemLog logs system-level messages (for system logs as requested)
func (l *Logger) SystemLog(format string, args ...interface{}) {
	// Create a constant format string
	const systemFormat = "[SYSTEM] %s"
	l.Log(systemFormat, fmt.Sprintf(format, args...))
}

// UserPrint logs and pretty prints user-facing messages
func (l *Logger) UserPrint(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)

	// Log it first
	l.Log("[USER_PRINT] %s", message)

	// Pretty print to console with formatting
	fmt.Println("\n" + formatMessage(message) + "\n")
}

// formatMessage adds pretty formatting to user messages
func formatMessage(message string) string {
	// Add border around the message
	border := "════════════════════════════════════════════════════════════════"
	return border + "\n" + message + "\n" + border
}

// LogError logs an error message
func (l *Logger) LogError(err error, context string) {
	l.Log("[ERROR] %s: %v", context, err)
}

// --- Raw Logging ---

func (l *Logger) logRaw(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] [%s] %s", timestamp, l.scope, message)
	l.rawLogger.Println(logEntry)
}

// LogAPIRequest logs API requests to the raw file
func (l *Logger) LogAPIRequest(payload string) {
	l.logRaw("[API_REQUEST] Payload: %s", payload)
}

// LogAPIResponse logs API responses to the raw file
func (l *Logger) LogAPIResponse(response any) {
	jsonBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		l.logRaw("[API_RESPONSE_ERROR] Failed to marshal response: %v", err)
		return
	}
	l.logRaw("[API_RESPONSE]\n%s", string(jsonBytes))
}

// --- Semantic Logging ---

// LogTurnStart demarcates the beginning of a turn
func (l *Logger) LogTurnStart(turnCount int) {
	l.Log("╭── TURN %d ─────────────────────────", turnCount)
}

// LogTurnEnd demarcates the end of a turn
func (l *Logger) LogTurnEnd() {
	l.Log("╰───────────────────────────────────")
	l.Log("")
}

// LogUserPrompt logs the user's prompt (or synthesized results)
func (l *Logger) LogUserPrompt(prompt string) {
	l.Log("│ 👤 User: %s", truncate(prompt, 200))
}

// LogLLMMessage logs the LLM's text response
func (l *Logger) LogLLMMessage(msg string) {
	if strings.TrimSpace(msg) != "" {
		l.Log("│ 🤖 LLM : %s", truncate(msg, 200))
	}
}

// LogToolCall logs a tool call made by the LLM
func (l *Logger) LogToolCall(name, args string) {
	l.Log("│ 🤖 LLM : [Tool Call] %s(%s)", name, truncate(args, 100))
}

// LogToolResult logs the execution result of a tool
func (l *Logger) LogToolResult(name, result string) {
	l.Log("│ 🛠️ Tool : [%s] Result: %s", name, truncate(result, 200))
}

// truncate helper
func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) > maxLen {
		return s[:maxLen] + " ... [truncated]"
	}
	return s
}
