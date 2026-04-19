package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// Logger handles both file logging and console output
type Logger struct {
	fileLogger *log.Logger
	logFile    *os.File
}

// NewLogger creates a new logger instance
func NewLogger() (*Logger, error) {
    // os.O_TRUNC wipes the file if it exists. 
    // os.O_CREATE creates it if it doesn't.
    file, err := os.OpenFile("common/logger/log.txt", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
    if err != nil {
        return nil, err
    }

    // Create a logger that writes to the file
    fileLogger := log.New(file, "", 0)

    return &Logger{
        fileLogger: fileLogger,
        logFile:    file,
    }, nil
}

// Close closes the log file
func (l *Logger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// Log writes a message to the log file with timestamp
func (l *Logger) Log(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] %s", timestamp, message)

	// Write to file
	l.fileLogger.Println(logEntry)
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

// LogAPIRequest logs API requests
func (l *Logger) LogAPIRequest(payload string) {
	l.Log("[API_REQUEST] Payload: %s", payload)
}

// LogAPIResponse logs API responses
func (l *Logger) LogAPIResponse(response any) {
	jsonBytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		l.Log("[API_RESPONSE_ERROR] Failed to marshal response: %v", err)
		return
	}
	l.Log("[API_RESPONSE] %s", string(jsonBytes))
}
