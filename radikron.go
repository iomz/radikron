package radikron

import (
	"context"
	"log"
	"time"
)

var (
	CurrentTime time.Time
	Location    *time.Location
)

func init() { //nolint:gochecknoinits
	var err error

	Location, err = time.LoadLocation(TZTokyo)
	if err != nil {
		log.Fatal(err)
	}
}

// EventEmitter defines the interface for emitting structured events.
// Implementations can provide structured events to external systems (e.g., GUI).
type EventEmitter interface {
	// EmitDownloadStarted emits when a download starts
	EmitDownloadStarted(stationID, title, startTime, uri string)
	// EmitDownloadCompleted emits when a download completes successfully (file written to disk)
	EmitDownloadCompleted(stationID, title, filePath string)
	// EmitFileSaved emits when a file is fully saved with metadata tags
	EmitFileSaved(stationID, title, filePath string)
	// EmitDownloadSkipped emits when a download is skipped (duplicate, already exists, etc.)
	EmitDownloadSkipped(reason string, stationID, title, startTime string)
	// EmitEncodingStarted emits when encoding to MP3 starts
	EmitEncodingStarted(filePath string)
	// EmitEncodingCompleted emits when encoding to MP3 completes successfully
	EmitEncodingCompleted(filePath string)
	// EmitLogMessage emits a general log message (for backward compatibility)
	EmitLogMessage(level string, message string)
}

// GetEventEmitter retrieves the EventEmitter from context, if available.
// Returns nil if no emitter is set in context (CLI mode).
func GetEventEmitter(ctx context.Context) EventEmitter {
	k := ContextKey("eventEmitter")
	emitter, ok := ctx.Value(k).(EventEmitter)
	if !ok {
		return nil
	}
	return emitter
}
