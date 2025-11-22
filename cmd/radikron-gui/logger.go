package main

import (
	"context"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
)

const (
	logTypeInfo    = "info"
	logTypeSuccess = "success"
	logTypeError   = "error"
	// Regex match group counts
	startDownloadMatchCount = 4 // [station], title, (time), :
	fileSavedMatchCount     = 2 // prefix, filepath
	minFilenameParts        = 3 // date, time, station (title is optional)
)

// EventLogger is a writer that emits log messages as Wails events
type EventLogger struct {
	ctx       context.Context
	emitEvent func(ctx context.Context, eventName string, data any)
}

// NewEventLogger creates a new EventLogger that emits log messages as Wails events
func NewEventLogger(ctx context.Context, emitEvent func(ctx context.Context, eventName string, data any)) *EventLogger {
	return &EventLogger{
		ctx:       ctx,
		emitEvent: emitEvent,
	}
}

// Write implements io.Writer interface
func (e *EventLogger) Write(p []byte) (n int, err error) {
	// Don't write to original writer here - MultiWriter already handles that
	// Just parse log message and emit event
	message := strings.TrimSpace(string(p))
	if message != "" {
		e.emitLogEvent(message)
	}

	// Return the length to indicate we "wrote" all bytes
	// The actual writing to stderr is handled by MultiWriter
	return len(p), nil
}

// emitLogEvent emits a log event based on the message content
func (e *EventLogger) emitLogEvent(message string) {
	// Determine log type based on message content
	logType := logTypeInfo

	// Parse "start downloading [station]title (time): uri" format
	// Example: "start downloading [ALPHA-STATION]NICE POP RADIO (20251121200000): https://..."
	startDownloadRe := regexp.MustCompile(`start downloading \[([^\]]+)\]([^(]+) \(([^)]+)\):`)
	if matches := startDownloadRe.FindStringSubmatch(message); len(matches) == startDownloadMatchCount {
		stationID := matches[1]
		title := strings.TrimSpace(matches[2])
		startTime := matches[3]

		// Emit download-started event
		e.emitEvent(e.ctx, "download-started", map[string]any{
			"station": stationID,
			"title":   title,
			"start":   startTime,
		})

		// Don't emit log-message for this - the download-started event is sufficient
		return
	}

	// Parse "+file saved: /path/to/file" format
	if strings.HasPrefix(message, "+file saved:") {
		// Extract filename to get program info
		// Format: YYYYMMDD_HHMMSS_stationID_title.ext
		fileSavedRe := regexp.MustCompile(`\+file saved: (.+)`)
		if matches := fileSavedRe.FindStringSubmatch(message); len(matches) == fileSavedMatchCount {
			filePath := matches[1]
			// Extract station and title from filename
			fileName := filePath
			if lastSlash := strings.LastIndex(filePath, "/"); lastSlash >= 0 {
				fileName = filePath[lastSlash+1:]
			}
			// Remove extension
			if extIndex := strings.LastIndex(fileName, "."); extIndex >= 0 {
				fileName = fileName[:extIndex]
			}
			// Split by underscore: [0]=date, [1]=time, [2]=station, [3+]=title
			parts := strings.Split(fileName, "_")
			if len(parts) >= minFilenameParts {
				stationID := parts[2]
				title := strings.Join(parts[3:], "_")

				// Emit download-completed event
				e.emitEvent(e.ctx, "download-completed", map[string]any{
					"station": stationID,
					"title":   title,
				})

				// Don't emit log-message for this - the download-completed event is sufficient
				return
			}
		}

		logType = logTypeSuccess
	} else if strings.HasPrefix(message, "-") {
		// Skip messages (already exists, duplicate, etc.)
		logType = logTypeInfo
	} else if strings.Contains(message, "failed") || strings.Contains(message, "error") {
		logType = logTypeError
	} else if strings.Contains(message, "start encoding") {
		logType = logTypeInfo
	}

	// Emit log-message event to frontend for all other messages
	e.emitEvent(e.ctx, "log-message", map[string]any{
		"type":    logType,
		"message": message,
	})
}

// SetupLogger sets up the custom logger to capture radikron log messages
func SetupLogger(ctx context.Context, emitEvent func(ctx context.Context, eventName string, data any)) func() {
	// Create event logger
	eventLogger := NewEventLogger(ctx, emitEvent)

	// Set log output to our custom writer
	// This will capture all log.Printf() calls
	log.SetOutput(io.MultiWriter(os.Stderr, eventLogger))

	// Return cleanup function to restore original log output
	return func() {
		log.SetOutput(os.Stderr)
	}
}
