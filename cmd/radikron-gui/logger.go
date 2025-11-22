package main

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/iomz/radikron"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	logTypeInfo      = "info"
	logTypeSuccess   = "success"
	logTypeError     = "error"
	minFilenameParts = 3 // date, time, station (title is optional)
)

// WailsEventEmitter implements radikron.EventEmitter interface using Wails runtime events
type WailsEventEmitter struct {
	ctx context.Context
}

// Ensure WailsEventEmitter implements radikron.EventEmitter at compile time
var _ radikron.EventEmitter = (*WailsEventEmitter)(nil)

// NewWailsEventEmitter creates a new WailsEventEmitter
func NewWailsEventEmitter(ctx context.Context) *WailsEventEmitter {
	return &WailsEventEmitter{ctx: ctx}
}

// EmitDownloadStarted implements radikron.EventEmitter
func (e *WailsEventEmitter) EmitDownloadStarted(stationID, title, startTime, uri string) {
	runtime.EventsEmit(e.ctx, "download-started", map[string]any{
		"station": stationID,
		"title":   title,
		"start":   startTime,
		"uri":     uri,
	})
}

// EmitDownloadCompleted implements radikron.EventEmitter
func (e *WailsEventEmitter) EmitDownloadCompleted(stationID, title, filePath string) {
	// Extract station and title from filePath if not provided
	if stationID == "" || title == "" {
		stationID, title = extractProgramInfoFromPath(filePath)
	}

	runtime.EventsEmit(e.ctx, "download-completed", map[string]any{
		"station": stationID,
		"title":   title,
	})
}

// EmitFileSaved implements radikron.EventEmitter
func (e *WailsEventEmitter) EmitFileSaved(stationID, title, filePath string) {
	// Extract station and title from filePath if not provided
	if stationID == "" || title == "" {
		stationID, title = extractProgramInfoFromPath(filePath)
	}

	runtime.EventsEmit(e.ctx, "file-saved", map[string]any{
		"station":  stationID,
		"title":    title,
		"filePath": filePath,
	})
}

// EmitDownloadSkipped implements radikron.EventEmitter
func (e *WailsEventEmitter) EmitDownloadSkipped(reason, stationID, title, startTime string) {
	runtime.EventsEmit(e.ctx, "download-skipped", map[string]any{
		"reason":  reason,
		"station": stationID,
		"title":   title,
		"start":   startTime,
	})
}

// EmitEncodingStarted implements radikron.EventEmitter
func (e *WailsEventEmitter) EmitEncodingStarted(filePath string) {
	runtime.EventsEmit(e.ctx, "encoding-started", map[string]any{
		"filePath": filePath,
	})
}

// EmitEncodingCompleted implements radikron.EventEmitter
func (e *WailsEventEmitter) EmitEncodingCompleted(filePath string) {
	runtime.EventsEmit(e.ctx, "encoding-completed", map[string]any{
		"filePath": filePath,
	})
}

// EmitLogMessage implements radikron.EventEmitter
func (e *WailsEventEmitter) EmitLogMessage(level, message string) {
	runtime.EventsEmit(e.ctx, "log-message", map[string]any{
		"type":    level,
		"message": message,
	})
}

// extractProgramInfoFromPath extracts station ID and title from a file path
func extractProgramInfoFromPath(filePath string) (stationID, title string) {
	fileName := filepath.Base(filePath)
	// Remove extension
	if extIndex := strings.LastIndex(fileName, "."); extIndex >= 0 {
		fileName = fileName[:extIndex]
	}
	// Split by underscore: [0]=date, [1]=time, [2]=station, [3+]=title
	parts := strings.Split(fileName, "_")
	if len(parts) >= minFilenameParts {
		stationID = parts[2]
		title = strings.Join(parts[3:], "_")
	}
	return stationID, title
}

// EventLogger is a writer that emits log messages as Wails events (fallback for non-structured logs)
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
	// Convert to lowercase for case-insensitive error detection
	lowerMessage := strings.ToLower(message)
	if strings.Contains(lowerMessage, "failed") || strings.Contains(lowerMessage, "error") {
		logType = logTypeError
	} else if strings.Contains(lowerMessage, "start encoding") {
		logType = logTypeInfo
	}

	// Emit log-message event to frontend for all other messages
	e.emitEvent(e.ctx, "log-message", map[string]any{
		"type":    logType,
		"message": message,
	})
}

// SetupLogger sets up the custom logger to capture radikron log messages
// Returns the event emitter and a cleanup function
func SetupLogger(
	ctx context.Context,
	emitEvent func(ctx context.Context, eventName string, data any),
) (emitter *WailsEventEmitter, cleanup func()) {
	// Create structured event emitter
	emitter = NewWailsEventEmitter(ctx)

	// Also create legacy event logger for backward compatibility (catches any log.Printf calls)
	eventLogger := NewEventLogger(ctx, emitEvent)

	// Set log output to our custom writer
	// This will capture all log.Printf() calls (fallback for non-structured logging)
	log.SetOutput(io.MultiWriter(os.Stderr, eventLogger))

	// Return event emitter and cleanup function to restore original log output
	cleanup = func() {
		log.SetOutput(os.Stderr)
	}
	return emitter, cleanup
}
