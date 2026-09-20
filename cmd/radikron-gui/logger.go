package main

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/iomz/radikron"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	logTypeInfo      = "info"
	logTypeSuccess   = "success"
	logTypeError     = "error"
	minFilenameParts = 3 // date, time, station (title is optional)
)

// DownloadCompletedCallback is called when a download completes
type DownloadCompletedCallback func(stationID, title, startTime string)

// WailsEventEmitter implements radikron.EventEmitter interface using Wails runtime events
type WailsEventEmitter struct {
	ctx                 context.Context
	emitEvent           func(context.Context, string, any)
	onDownloadCompleted DownloadCompletedCallback
	callbackMu          sync.Mutex
}

// Ensure WailsEventEmitter implements radikron.EventEmitter at compile time
var _ radikron.EventEmitter = (*WailsEventEmitter)(nil)

// NewWailsEventEmitter creates a new WailsEventEmitter
func NewWailsEventEmitter(ctx context.Context) *WailsEventEmitter {
	return &WailsEventEmitter{
		ctx: ctx,
		emitEvent: func(ctx context.Context, eventName string, data any) {
			runtime.EventsEmit(ctx, eventName, data)
		},
	}
}

func (e *WailsEventEmitter) emit(eventName string, data any) {
	if e.emitEvent != nil {
		e.emitEvent(e.ctx, eventName, data)
	}
}

// SetDownloadCompletedCallback sets the callback for download completion
func (e *WailsEventEmitter) SetDownloadCompletedCallback(callback DownloadCompletedCallback) {
	e.callbackMu.Lock()
	defer e.callbackMu.Unlock()
	e.onDownloadCompleted = callback
}

// EmitDownloadStarted implements radikron.EventEmitter
func (e *WailsEventEmitter) EmitDownloadStarted(stationID, title, startTime string) {
	log.Printf("start downloading [%s]%s (%s)", stationID, title, startTime)
	e.emit("download-started", map[string]any{
		"station": stationID,
		"title":   title,
		"start":   startTime,
	})
}

// EmitDownloadCompleted implements radikron.EventEmitter
func (e *WailsEventEmitter) EmitDownloadCompleted(stationID, title, startTime, filePath string) {
	// Extract station and title from filePath if not provided
	if stationID == "" || title == "" {
		stationID, title = extractProgramInfoFromPath(filePath)
	}

	log.Printf("download completed [%s]%s: %s", stationID, title, filePath)
	e.emit("download-completed", map[string]any{
		"station": stationID,
		"title":   title,
		"start":   startTime,
	})

	// Call callback if set (for handling manual injections)
	e.callbackMu.Lock()
	callback := e.onDownloadCompleted
	e.callbackMu.Unlock()
	if callback != nil {
		callback(stationID, title, startTime)
	}
}

// EmitFileSaved implements radikron.EventEmitter
func (e *WailsEventEmitter) EmitFileSaved(stationID, title, filePath string) {
	// Extract station and title from filePath if not provided
	if stationID == "" || title == "" {
		stationID, title = extractProgramInfoFromPath(filePath)
	}

	log.Printf("+file saved: %s", filePath)
	e.emit("file-saved", map[string]any{
		"station":  stationID,
		"title":    title,
		"filePath": filePath,
	})
}

// EmitDownloadSkipped implements radikron.EventEmitter
func (e *WailsEventEmitter) EmitDownloadSkipped(reason, stationID, title, startTime string) {
	if stationID != "" && title != "" && startTime != "" {
		log.Printf("-skip %s [%s]%s (%s)", reason, stationID, title, startTime)
	} else {
		log.Printf("-skip %s", reason)
	}
	e.emit("download-skipped", map[string]any{
		"reason":  reason,
		"station": stationID,
		"title":   title,
		"start":   startTime,
	})
}

// EmitEncodingStarted implements radikron.EventEmitter
func (e *WailsEventEmitter) EmitEncodingStarted(filePath string) {
	log.Printf("start encoding to MP3: %s", filePath)
	e.emit("encoding-started", map[string]any{
		"filePath": filePath,
	})
}

// EmitEncodingCompleted implements radikron.EventEmitter
func (e *WailsEventEmitter) EmitEncodingCompleted(filePath string) {
	log.Printf("finish encoding to MP3: %s", filePath)
	e.emit("encoding-completed", map[string]any{
		"filePath": filePath,
	})
}

// EmitLogMessage implements radikron.EventEmitter
func (e *WailsEventEmitter) EmitLogMessage(level, message string) {
	e.emit("log-message", map[string]any{
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
	// Current names use [datetime]_[station]_[title]. Keep legacy
	// [date]_[time]_[station]_[title] parsing for existing downloads.
	parts := strings.Split(fileName, "_")
	if len(parts) >= minFilenameParts {
		if _, err := time.Parse(radikron.OutputDatetimeLayout, parts[0]); err == nil {
			stationID = parts[1]
			title = strings.Join(parts[2:], "_")
			return stationID, title
		}
	}
	if len(parts) >= minFilenameParts+1 {
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
