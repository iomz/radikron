package radikron

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bogem/id3v2"
	"github.com/grafov/m3u8"
	"github.com/yyoshiki41/radigo"
)

var (
	downloadingSem = make(chan struct{}, MaxDownloadingConcurrency)
	encodingSem    = make(chan struct{}, MaxEncodingConcurrency)
	semMu          sync.Mutex // protects semaphore recreation
)

const (
	timeshiftDebugEnv         = "RADIKRON_TIMESHIFT_DEBUG"
	timeshiftChunklistDumpEnv = "RADIKRON_TIMESHIFT_CHUNKLIST_DUMP"
	timeshiftDiagnosticPerm   = 0600
)

// emitDownloadStarted emits a download started event if emitter is available, otherwise logs it
func emitDownloadStarted(ctx context.Context, stationID, title, startTime string) {
	if emitter := GetEventEmitter(ctx); emitter != nil {
		emitter.EmitDownloadStarted(stationID, title, startTime)
	} else {
		log.Printf("start downloading [%s]%s (%s)", stationID, title, startTime)
	}
}

// emitDownloadCompleted emits a download completed event if emitter is available, otherwise logs it
func emitDownloadCompleted(ctx context.Context, stationID, title, startTime, filePath string) {
	if emitter := GetEventEmitter(ctx); emitter != nil {
		emitter.EmitDownloadCompleted(stationID, title, startTime, filePath)
	} else {
		log.Printf("download completed [%s]%s: %s", stationID, title, filePath)
	}
}

// emitFileSaved emits a file saved event if emitter is available, otherwise logs it
func emitFileSaved(ctx context.Context, stationID, title, filePath string) {
	if emitter := GetEventEmitter(ctx); emitter != nil {
		emitter.EmitFileSaved(stationID, title, filePath)
	} else {
		log.Printf("+file saved: %s", filePath)
	}
}

// emitDownloadSkipped emits a download skipped event if emitter is available, otherwise logs it
func emitDownloadSkipped(ctx context.Context, reason, stationID, title, startTime string) {
	if emitter := GetEventEmitter(ctx); emitter != nil {
		emitter.EmitDownloadSkipped(reason, stationID, title, startTime)
	} else {
		if stationID != "" && title != "" && startTime != "" {
			log.Printf("-skip %s [%s]%s (%s)", reason, stationID, title, startTime)
		} else {
			log.Printf("-skip %s", reason)
		}
	}
}

// emitEncodingStarted emits an encoding started event if emitter is available, otherwise logs it
func emitEncodingStarted(ctx context.Context, filePath string) {
	if emitter := GetEventEmitter(ctx); emitter != nil {
		emitter.EmitEncodingStarted(filePath)
	} else {
		log.Printf("start encoding to MP3: %s", filePath)
	}
}

// emitEncodingCompleted emits an encoding completed event if emitter is available, otherwise logs it
func emitEncodingCompleted(ctx context.Context, filePath string) {
	if emitter := GetEventEmitter(ctx); emitter != nil {
		emitter.EmitEncodingCompleted(filePath)
	} else {
		log.Printf("finish encoding to MP3: %s", filePath)
	}
}

// emitLogMessage emits a log message if emitter is available, otherwise logs it
func emitLogMessage(ctx context.Context, level, message string) {
	if emitter := GetEventEmitter(ctx); emitter != nil {
		emitter.EmitLogMessage(level, message)
	} else {
		// Use default level if empty or missing
		if level == "" {
			level = "INFO"
		}
		log.Printf("[%s] %s", strings.ToUpper(level), message)
	}
}

// InitSemaphores initializes or updates the semaphores based on the asset's concurrency settings.
// This should be called when configuration is applied to ensure semaphores match the config.
func InitSemaphores(asset *Asset) {
	if asset == nil {
		return
	}

	maxDownloadingConcurrency := asset.MaxDownloadingConcurrency
	if maxDownloadingConcurrency <= 0 {
		maxDownloadingConcurrency = MaxDownloadingConcurrency
	}

	maxEncodingConcurrency := asset.MaxEncodingConcurrency
	if maxEncodingConcurrency <= 0 {
		maxEncodingConcurrency = MaxEncodingConcurrency
	}

	semMu.Lock()
	defer semMu.Unlock()

	// Recreate semaphores if values changed
	if cap(downloadingSem) != maxDownloadingConcurrency {
		downloadingSem = make(chan struct{}, maxDownloadingConcurrency)
	}
	if cap(encodingSem) != maxEncodingConcurrency {
		encodingSem = make(chan struct{}, maxEncodingConcurrency)
	}
}

// errSkipAfterMove is a sentinel error indicating the file was moved and exists at target,
// so download should be skipped without logging "skip already exists"
var errSkipAfterMove = errors.New("skip after move")

// checkFutureProgram checks if the program is in the future and handles it accordingly.
// Returns true if the program is in the future (and was handled), false otherwise.
func checkFutureProgram(ctx context.Context, asset *Asset, prog *Prog, startTime time.Time, title, start string) (bool, error) {
	if !startTime.After(CurrentTime) {
		return false, nil
	}

	nextEndTime, err := time.ParseInLocation(DatetimeLayout, prog.To, Location)
	if err != nil {
		emitLogMessage(ctx, "error", fmt.Sprintf("Failed to parse end time '%s': %v", prog.To, err))
		return true, fmt.Errorf("invalid end time format '%s': %w", prog.To, err)
	}

	// update the next fetching time
	if asset.NextFetchTime == nil || asset.NextFetchTime.After(nextEndTime) {
		next := nextEndTime.Add(BufferMinutes * time.Minute)
		asset.NextFetchTime = &next
	}

	msg := fmt.Sprintf("skipping future program [%s]%s (starts at %s, current time %s)",
		prog.StationID, title, start, CurrentTime.Format(DatetimeLayout))
	emitLogMessage(ctx, "info", msg)
	return true, nil
}

// checkDuplicateInSchedules checks if the program is already in schedules.
// Returns true if duplicate (and was handled), false otherwise.
func checkDuplicateInSchedules(ctx context.Context, asset *Asset, prog *Prog, title, start string) bool {
	if !asset.Schedules.HasDuplicate(prog) {
		return false
	}

	msg := fmt.Sprintf("duplicate program already in schedules, skipping [%s]%s (%s)", prog.StationID, title, start)
	emitDownloadSkipped(ctx, "duplicate program", prog.StationID, title, start)
	emitLogMessage(ctx, "info", msg)
	return true
}

// setupOutputConfig creates and sets up the output configuration.
func setupOutputConfig(ctx context.Context, asset *Asset, prog *Prog, startTime time.Time) (*radigo.OutputConfig, error) {
	fileBaseName := fmt.Sprintf(
		"%s_%s_%s",
		startTime.In(Location).Format(OutputDatetimeLayout),
		prog.StationID,
		prog.Title,
	)

	output, err := NewOutputConfig(
		fileBaseName,
		asset.OutputFormat,
		asset.DownloadDir,
		prog.RuleFolder,
	)
	if err != nil {
		emitLogMessage(ctx, "error", fmt.Sprintf("Failed to configure output: %v", err))
		return nil, fmt.Errorf("failed to configure output: %w", err)
	}

	if err = output.SetupDir(); err != nil {
		emitLogMessage(ctx, "error", fmt.Sprintf("Failed to setup output dir: %v", err))
		return nil, fmt.Errorf("failed to setup the output dir: %w", err)
	}

	return output, nil
}

// checkFileExists checks if the output file already exists and handles it.
// Returns true if file exists (and was handled), false otherwise.
func checkFileExists(ctx context.Context, output *radigo.OutputConfig, prog *Prog, start string) bool {
	if !output.IsExist() {
		return false
	}

	msg := fmt.Sprintf("file already exists at target, skipping [%s]%s: %s", prog.StationID, prog.Title, output.AbsPath())
	emitDownloadSkipped(ctx, "already exists", prog.StationID, prog.Title, start)
	emitLogMessage(ctx, "info", msg)
	return true
}

func Download(
	ctx context.Context,
	wg *sync.WaitGroup,
	prog *Prog,
) (err error) {
	asset := GetAsset(ctx)
	if asset == nil {
		emitLogMessage(ctx, "error", "Asset is nil in Download context")
		return fmt.Errorf("asset is nil in context")
	}
	title := prog.Title
	start := prog.Ft
	var startTime time.Time

	startTime, err = time.ParseInLocation(DatetimeLayout, start, Location)
	if err != nil {
		emitLogMessage(ctx, "error", fmt.Sprintf("Failed to parse start time '%s': %v", start, err))
		return fmt.Errorf("invalid start time format '%s': %w", start, err)
	}

	// Check if program is in the future
	if handled, err := checkFutureProgram(ctx, asset, prog, startTime, title, start); err != nil {
		return err
	} else if handled {
		return nil
	}

	// Check for duplicate in schedules (for direct calls to Download, e.g., in CLI mode or tests)
	// Note: In GUI mode, processProgram() adds programs to schedules before calling Download(),
	// but we still need this check for CLI mode and tests where Download() is called directly.
	if checkDuplicateInSchedules(ctx, asset, prog, title, start) {
		return nil
	}

	// Setup output configuration
	output, err := setupOutputConfig(ctx, asset, prog, startTime)
	if err != nil {
		return err
	}

	// Check if file already exists
	if checkFileExists(ctx, output, prog, start) {
		return nil
	}

	fileBaseName := fmt.Sprintf(
		"%s_%s_%s",
		startTime.In(Location).Format(OutputDatetimeLayout),
		prog.StationID,
		title,
	)

	// Check for duplicates and move from default folder to configured folder if needed
	// handleDuplicate checks other locations and handles skip cases
	if err := handleDuplicate(
		ctx, fileBaseName, asset.OutputFormat, asset.DownloadDir,
		prog.RuleFolder, output, asset.Rules, prog.StationID, title, start); err != nil {
		// If errSkipAfterMove, file was moved and exists at target - skip without logging again
		if errors.Is(err, errSkipAfterMove) {
			return nil
		}
		emitLogMessage(ctx, "error", fmt.Sprintf("Failed to handle duplicate: %v", err))
		return fmt.Errorf("failed to handle duplicate: %w", err)
	}

	// Log rule match only when download actually starts (not skipped)
	if prog.RuleName != "" {
		emitLogMessage(ctx, "info", fmt.Sprintf("rule[%s] matched: [%s]%s (%s)", prog.RuleName, prog.StationID, title, start))
	}
	emitDownloadStarted(ctx, prog.StationID, title, start)
	wg.Add(1)
	go downloadProgram(ctx, wg, prog, output)
	return nil
}

func bulkDownload(list []string, output string) error {
	var (
		errFlag bool
		mu      sync.Mutex
	)
	var wg sync.WaitGroup

	for _, v := range list {
		wg.Add(1)
		go func(link string) {
			defer wg.Done()

			var err error
			for range MaxRetryAttempts {
				downloadingSem <- struct{}{}
				err = downloadLink(link, output)
				<-downloadingSem
				if err == nil {
					break
				}
			}
			if err != nil {
				log.Printf("failed to download: %s", err)
				mu.Lock()
				errFlag = true
				mu.Unlock()
			}
		}(v)
	}
	wg.Wait()

	mu.Lock()
	hasError := errFlag
	mu.Unlock()

	if hasError {
		return errors.New("lack of aac files")
	}
	return nil
}

func downloadLink(link, output string) error {
	resp, err := http.Get(link) //nolint:gosec,noctx
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, fileName := filepath.Split(link)
	file, err := os.Create(filepath.Join(output, fileName))
	if err != nil {
		return err
	}

	_, err = io.Copy(file, resp.Body)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

// downloadProgram manages the download for the given program
// in a go routine and notify the wg when finished
func downloadProgram(
	ctx context.Context, // the context for the request
	wg *sync.WaitGroup, // the wg to notify
	prog *Prog, // the program metadata
	output *radigo.OutputConfig, // the file configuration
) {
	defer wg.Done()
	var err error

	chunklist, err := getTimeshiftChunklist(ctx, prog)
	if err != nil {
		log.Printf("failed to get chunklist: %s", err)
		return
	}

	aacDir, err := tempAACDir()
	if err != nil {
		log.Printf("failed to create the aac dir: %s", err)
		return
	}
	defer os.RemoveAll(aacDir) // clean up

	if err = bulkDownload(chunklist, aacDir); err != nil {
		log.Printf("failed to download aac files: %s", err)
		return
	}

	// Download completed - tmp files are ready for concatenation and validation
	emitDownloadCompleted(ctx, prog.StationID, prog.Title, prog.Ft, output.AbsPath())

	concatedFile, err := radigo.ConcatAACFilesFromList(ctx, aacDir)
	if err != nil {
		log.Printf("failed to concat aac files: %s", err)
		return
	}

	if err = writeOutputFile(ctx, concatedFile, output); err != nil {
		log.Printf("failed to write the output file: %s", err)
		return
	}

	if shouldRetry := validateAndCleanupOutputFile(ctx, output); shouldRetry {
		return
	}

	err = writeID3Tag(output, prog)
	if err != nil {
		emitLogMessage(ctx, "error", fmt.Sprintf("ID3v2: %v", err))
		return
	}

	// File saved - metadata tags have been written
	emitFileSaved(ctx, prog.StationID, prog.Title, output.AbsPath())
}

// writeOutputFile writes the concatenated file to the output location,
// handling format conversion (AAC to MP3) if needed.
func writeOutputFile(ctx context.Context, concatedFile string, output *radigo.OutputConfig) error {
	switch output.AudioFormat() {
	case radigo.AudioFormatAAC:
		return moveFile(concatedFile, output.AbsPath())
	case radigo.AudioFormatMP3:
		// Limit concurrent encoding operations to prevent resource exhaustion
		encodingSem <- struct{}{}
		defer func() { <-encodingSem }()
		emitEncodingStarted(ctx, output.AbsPath())
		err := convertAACtoMP3(ctx, concatedFile, output.AbsPath())
		if err == nil {
			emitEncodingCompleted(ctx, output.AbsPath())
		}
		return err
	default:
		return fmt.Errorf("invalid file format")
	}
}

// validateAndCleanupOutputFile validates the output file size and removes it
// if it's too small, scheduling a retry. Returns true if a retry was scheduled.
func validateAndCleanupOutputFile(ctx context.Context, output *radigo.OutputConfig) bool {
	info, err := os.Stat(output.AbsPath())
	if err != nil {
		log.Printf("failed to stat the output file: %s", err)
		return false
	}

	asset := GetAsset(ctx)
	if info.Size() < asset.MinimumOutputSize {
		log.Printf("the output file is too small: %v MB", float32(info.Size())/Kilobytes/Kilobytes)
		err = os.Remove(output.AbsPath())
		if err != nil {
			log.Printf("failed to remove the file: %v", err)
			return false
		}
		next := time.Now().In(Location).Add(BufferMinutes * time.Minute)
		asset.NextFetchTime = &next
		log.Printf("removed the file, retry downloading at %v", next)
		return true
	}
	return false
}

// convertAACtoMP3 converts an AAC file to MP3 format using ffmpeg.
func convertAACtoMP3(ctx context.Context, sourceFile, destFile string) error {
	// Check if ffmpeg is available
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	// Build ffmpeg command:
	// -i: input file
	// -acodec libmp3lame: use MP3 codec
	// -ar 44100: sample rate 44.1kHz
	// -y: overwrite output file if it exists
	// -loglevel error: only show errors
	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-i", sourceFile,
		"-acodec", "libmp3lame",
		"-map_metadata", "0",
		"-ar", "44100",
		"-y",
		"-loglevel", "error",
		destFile,
	)

	// Capture stderr for error messages
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg conversion failed: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}

// getTimeshiftChunklist returns a slice of chunk urls.
//
//nolint:gocyclo,funlen // keep this function monolithic
func getTimeshiftChunklist(
	ctx context.Context,
	prog *Prog,
) ([]string, error) {
	const (
		seekStep       = 15 * time.Second
		playlistLength = "20"
	)

	debugEnvValue := os.Getenv(timeshiftDebugEnv)
	dumpEnvValue := os.Getenv(timeshiftChunklistDumpEnv)
	debugTimeshift := timeshiftDebugEnabled()
	dumpEnabled := dumpEnvValue != ""
	if debugTimeshift || dumpEnabled {
		log.Printf( //nolint:gosec // diagnostic environment values are quoted
			"timeshift diagnostics debug_env=%q dump_env=%q debug_enabled=%t dump_enabled=%t",
			debugEnvValue,
			dumpEnvValue,
			debugTimeshift,
			dumpEnabled,
		)
	}

	var dumpPath string
	if dumpEnabled {
		absoluteDumpPath, err := filepath.Abs(dumpEnvValue)
		if err != nil {
			log.Printf( //nolint:gosec // developer-supplied diagnostic path is quoted
				"timeshift chunklist dump path resolution failed path=%q: %v",
				dumpEnvValue,
				err,
			)
		} else {
			dumpPath = absoluteDumpPath
			log.Printf( //nolint:gosec // developer-supplied diagnostic path is quoted
				"timeshift chunklist dump enabled path=%q",
				dumpPath,
			)
		}
	}

	asset := GetAsset(ctx)
	client := asset.DefaultClient
	var err error

	areaID := asset.GetAreaIDByStationID(prog.StationID)

	device, ok := asset.AreaDevices[areaID]
	if !ok {
		device, err = asset.NewDevice(ctx, areaID)
		if err != nil {
			return nil, err
		}
	}

	location, err := time.LoadLocation(TZTokyo)
	if err != nil {
		return nil, err
	}

	ft, err := time.ParseInLocation(DatetimeLayout, prog.Ft, location)
	if err != nil {
		return nil, err
	}
	to, err := time.ParseInLocation(DatetimeLayout, prog.To, location)
	if err != nil {
		return nil, err
	}
	if !to.After(ft) {
		return nil, fmt.Errorf("invalid program range: ft=%s to=%s", prog.Ft, prog.To)
	}

	seen := map[string]bool{}
	var chunklist []string

	// Overlap requests because HLS segment boundaries may not align with fixed seek intervals.
	for seek := ft; seek.Before(to); seek = seek.Add(seekStep) {
		// build m3u8 request uri
		u, err := url.Parse(APIPlaylistM3U8)
		if err != nil {
			log.Fatal(err)
		}

		// set query parameters
		q := u.Query()
		q.Set("station_id", prog.StationID)
		q.Set("start_at", prog.Ft)
		q.Set("ft", prog.Ft)
		q.Set("end_at", prog.To)
		q.Set("to", prog.To)
		seekValue := seek.In(location).Format(DatetimeLayout)
		q.Set("seek", seekValue)
		q.Set("preroll", "0")
		q.Set("l", playlistLength)
		q.Set("lsid", generateLSID())
		q.Set("type", "b")
		u.RawQuery = q.Encode()
		if debugTimeshift {
			log.Printf("timeshift seek=%s", seekValue)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
		if err != nil {
			return nil, err
		}
		req = req.WithContext(ctx)
		req.Header.Set("pragma", "no-cache")
		req.Header.Set(UserAgentHeader, device.UserAgent)
		req.Header.Set(RadikoAreaIDHeader, areaID)
		req.Header.Set(RadikoAuthTokenHeader, device.AuthToken)

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request failed (%s): %v", u.String(), err)
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read body failed (%s): %v", u.String(), readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("%s -> %s", u.String(), resp.Status)
		}

		playlistURI, err := parseMasterPlaylistURI(string(body))
		if err != nil {
			return nil, fmt.Errorf("%s -> %v", u.String(), err)
		}

		mediaPlaylistURL := resolveURL(u.String(), playlistURI)
		if debugTimeshift {
			log.Printf("timeshift media playlist seek=%s url=%q", seekValue, mediaPlaylistURL)
		}
		chunks, err := parseChunklistFromM3U8(mediaPlaylistURL)
		if err != nil {
			return nil, fmt.Errorf("%s -> chunklist parse failed: %v", u.String(), err)
		}
		for _, c := range chunks {
			segment, parseErr := parseTimeshiftProgramSegment(c, prog.StationID, ft, to, location)
			if parseErr != nil {
				if debugTimeshift {
					log.Printf(
						"timeshift chunk skipped seek=%s url=%q reason=%q",
						seekValue,
						c,
						parseErr,
					)
				}
				continue
			}

			key := segment.key()
			duplicate := seen[key]
			if debugTimeshift {
				log.Printf(
					"timeshift chunk seek=%s url=%q key=%q timestamp=%s duplicate=%t",
					seekValue,
					c,
					key,
					segment.Timestamp.Format(DatetimeLayout),
					duplicate,
				)
			}
			if !duplicate {
				seen[key] = true
				chunklist = append(chunklist, c)
			}
		}
	}
	if debugTimeshift {
		log.Printf("timeshift final chunk count=%d", len(chunklist))
	}
	if dumpPath != "" {
		if err := dumpChunklist(dumpPath, chunklist); err != nil {
			log.Printf( //nolint:gosec // developer-supplied diagnostic path is quoted
				"timeshift chunklist dump failed path=%q: %v",
				dumpPath,
				err,
			)
		} else {
			log.Printf( //nolint:gosec // developer-supplied diagnostic path is quoted
				"timeshift chunklist dumped path=%q count=%d",
				dumpPath,
				len(chunklist),
			)
		}
	}
	return chunklist, nil
}

// GetRadikronPath resolves a provided path (or defaults to the user's Downloads/radiko directory).
// If a relative path is provided, it's resolved relative to the current working directory.
// If an absolute path is provided, it's used as-is.
// If no path is provided, it defaults to the user's Downloads/radiko directory,
// with a fallback to the current working directory if the home directory cannot be determined.
func GetRadikronPath(path string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	switch {
	case path != "" && !filepath.IsAbs(path):
		// Relative path - need working directory
		path = filepath.Join(cwd, path)
	case path != "" && filepath.IsAbs(path):
		// Absolute path - no need for working directory
	default:
		// Default path - use user's Downloads/radiko directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			// Fallback to current working directory if home directory can't be determined
			path = filepath.Join(cwd, "radiko")
		} else {
			path = filepath.Join(homeDir, "Downloads", "radiko")
		}
	}
	return filepath.Clean(path), nil
}

// newOutputConfigFromPath creates an OutputConfig from a directory path, file base name, and format.
func newOutputConfigFromPath(dirPath, fileBaseName, fileFormat string) *radigo.OutputConfig {
	return &radigo.OutputConfig{
		DirFullPath:  dirPath,
		FileBaseName: fileBaseName,
		FileFormat:   fileFormat,
	}
}

// moveFile attempts to move a file using os.Rename, falling back to copy-then-delete
// if the rename fails (e.g., across filesystems).
func moveFile(source, dest string) error {
	// First attempt: try os.Rename (atomic and fast on same filesystem)
	err := os.Rename(source, dest)
	if err == nil {
		return nil
	}

	// Fallback: copy-then-delete (handles cross-filesystem moves)
	srcFile, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}

	_, err = io.Copy(destFile, srcFile)
	if closeErr := destFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		// Clean up destination file if copy failed
		_ = os.Remove(dest)
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Delete source file after successful copy. Once the destination has been
	// written and closed, cleanup failure must not make the user-visible output
	// look failed.
	if err := os.Remove(source); err != nil {
		log.Printf("warning: failed to remove source file after copy: %v", err)
	}

	return nil
}

// collectConfiguredFolders collects all unique configured folders from rules and the current configured folder
func collectConfiguredFolders(configuredFolder string, rules Rules) map[string]bool {
	configuredFolders := make(map[string]bool)
	if configuredFolder != "" {
		configuredFolders[configuredFolder] = true
	}
	for _, rule := range rules {
		if rule.Folder != "" {
			configuredFolders[rule.Folder] = true
		}
	}
	return configuredFolders
}

// checkConfiguredFoldersForDuplicate checks if file exists in any configured folder (excluding target)
func checkConfiguredFoldersForDuplicate(
	configuredFolders map[string]bool,
	downloadDir, fileBaseName, fileFormat, targetPath string,
) (exists bool, existingPath string) {
	for folder := range configuredFolders {
		configuredPath, err := GetRadikronPath(filepath.Join(downloadDir, folder))
		if err != nil {
			continue
		}
		configuredOutput := newOutputConfigFromPath(configuredPath, fileBaseName, fileFormat)
		// Skip if this is the target location (already checked above)
		if configuredOutput.AbsPath() == targetPath {
			continue
		}
		if configuredOutput.IsExist() {
			return true, configuredOutput.AbsPath()
		}
	}
	return false, ""
}

// handleMoveFromDefaultFolder handles moving a file from default folder to configured folder
func handleMoveFromDefaultFolder(
	ctx context.Context,
	source, targetPath string,
	output *radigo.OutputConfig,
	stationID, title, startTime string,
) error {
	// Check if target already exists (edge case: file appeared between checks or race condition)
	if output.IsExist() {
		emitDownloadSkipped(ctx, "target already exists, keeping both files", stationID, title, startTime)
		emitLogMessage(ctx, "info", fmt.Sprintf("target already exists, keeping both files: %s (target: %s)", source, targetPath))
		// Target exists, keep both files and skip download
		return nil
	}

	if err := moveFile(source, targetPath); err != nil {
		// Check if error is due to target existing (race condition during move)
		if _, statErr := os.Stat(targetPath); statErr == nil {
			log.Printf("warning: target file appeared during move, removing source: %s (target: %s)", source, targetPath)
			_ = os.Remove(source)
			return nil
		}
		return fmt.Errorf("failed to move file from default to configured folder (%s -> %s): %w", source, targetPath, err)
	}

	log.Printf("moved file: %s -> %s", source, targetPath)
	// After successful move, file exists at target - skip download
	// Return sentinel error to signal skip without logging "skip already exists"
	if output.IsExist() {
		return errSkipAfterMove
	}
	return nil
}

// handleDuplicate checks for duplicates in all configured folders and moves files from default folder to configured folder if needed
func handleDuplicate(
	ctx context.Context,
	fileBaseName, fileFormat, downloadDir, configuredFolder string,
	output *radigo.OutputConfig,
	rules Rules,
	stationID, title, startTime string,
) error {
	// Collect all unique configured folders from all rules
	configuredFolders := collectConfiguredFolders(configuredFolder, rules)

	// Check in all configured folders (excluding target, which is already checked before calling this)
	// This takes precedence over default folder
	targetPath := output.AbsPath()
	exists, existingPath := checkConfiguredFoldersForDuplicate(
		configuredFolders, downloadDir, fileBaseName, fileFormat, targetPath)
	if exists {
		emitDownloadSkipped(ctx, "already exists", stationID, title, startTime)
		emitLogMessage(ctx, "info", fmt.Sprintf("file already exists in configured folder, skipping [%s]%s: %s", stationID, title, existingPath))
		return nil
	}

	// Check in default download directory
	defaultPath, err := GetRadikronPath(downloadDir)
	if err != nil {
		return nil
	}
	defaultOutput := newOutputConfigFromPath(defaultPath, fileBaseName, fileFormat)
	if !defaultOutput.IsExist() {
		return nil
	}

	// If file exists in default folder and there's a configured folder, move it
	if configuredFolder != "" {
		return handleMoveFromDefaultFolder(ctx, defaultOutput.AbsPath(), output.AbsPath(), output, stationID, title, startTime)
	}

	// File exists in default folder, no configured folder - skip
	emitDownloadSkipped(ctx, "already exists", stationID, title, startTime)
	emitLogMessage(ctx, "info", fmt.Sprintf(
		"file already exists in default folder, skipping [%s]%s: %s",
		stationID, title, defaultOutput.AbsPath()))
	return nil
}

// NewOutputConfig prepares the outputdir
func NewOutputConfig(fileBaseName, fileFormat, downloadDir, folder string) (*radigo.OutputConfig, error) {
	basePath := downloadDir
	if folder != "" {
		basePath = filepath.Join(downloadDir, folder)
	}
	fullPath, err := GetRadikronPath(basePath)
	if err != nil {
		return nil, err
	}

	return &radigo.OutputConfig{
		DirFullPath:  fullPath,
		FileBaseName: fileBaseName,
		FileFormat:   fileFormat,
	}, nil
}

// tempAACDir creates a dir to store temporary aac files
func tempAACDir() (string, error) {
	// Use system temporary directory
	tmpDir := os.TempDir()

	aacDir, err := os.MkdirTemp(tmpDir, "radikron-aac-")
	if err != nil {
		return "", err
	}

	return aacDir, nil
}

func writeID3Tag(output *radigo.OutputConfig, prog *Prog) error {
	tag, err := id3v2.Open(output.AbsPath(), id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("error while opening the output file: %w", err)
	}
	defer tag.Close()

	// Set tags
	tag.SetTitle(output.FileBaseName)
	tag.SetArtist(prog.Pfm)
	tag.SetAlbum(prog.Title)
	tag.SetYear(prog.Ft[:4])

	// Add comment with program info
	tag.AddCommentFrame(id3v2.CommentFrame{
		Encoding:    id3v2.EncodingUTF8,
		Language:    ID3v2LangJPN,
		Description: prog.Info,
	})

	// Set rule name as Band/Orchestra/Accompaniment (TPE2) if available
	// Note: Many music players display TPE2 as "Album Artist"
	if prog.RuleName != "" {
		tag.AddTextFrame(tag.CommonID("Band/Orchestra/Accompaniment"), id3v2.EncodingUTF8, prog.RuleName)
	}

	// write tag to the aac
	if err = tag.Save(); err != nil {
		return fmt.Errorf("error while saving a tag: %w", err)
	}

	return nil
}

func generateLSID() string {
	b := make([]byte, 16) //nolint:mnd // 16 bytes → 32 hex chars
	if _, err := rand.Read(b); err != nil {
		log.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func extractChunklist(input io.Reader) ([]string, error) {
	playlist, listType, err := m3u8.DecodeFrom(input, true)
	if err != nil || listType != m3u8.MEDIA {
		return nil, err
	}
	p := playlist.(*m3u8.MediaPlaylist)

	var chunklist []string
	for _, v := range p.Segments {
		if v != nil {
			chunklist = append(chunklist, v.URI)
		}
	}
	return chunklist, nil
}

func parseChunklistFromM3U8(uri string) ([]string, error) {
	resp, err := http.Get(uri) //nolint:gosec,noctx
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return extractChunklist(resp.Body)
}

func parseMasterPlaylistURI(body string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(body))
	expectURI := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			expectURI = true
			continue
		}
		if expectURI && !strings.HasPrefix(line, "#") {
			return line, nil
		}
	}
	if strings.Contains(body, "#EXTM3U") {
		return "", fmt.Errorf("master playlist uri not found")
	}
	return "", fmt.Errorf("invalid m3u8 body")
}

func resolveURL(baseURL, ref string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return ref
	}
	uri, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(uri).String()
}

type timeshiftProgramSegment struct {
	Path      string
	StationID string
	Date      string
	Timestamp time.Time
}

type timeshiftSegmentPath struct {
	Path      string
	StationID string
	Date      string
	Filename  string
}

// The random filename suffix may vary between playlist responses.
// Use station + timestamp as the stable media segment identity.
func (s timeshiftProgramSegment) key() string {
	return s.StationID + "/" + s.Timestamp.Format(DatetimeLayout)
}

func parseTimeshiftSegmentPath(raw string) (timeshiftSegmentPath, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return timeshiftSegmentPath{}, fmt.Errorf("parse url: %w", err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	segmentsIndex := -1
	for i := range parts {
		if parts[i] == "segments" {
			segmentsIndex = i
			break
		}
	}
	if segmentsIndex == -1 || len(parts) <= segmentsIndex+1 {
		return timeshiftSegmentPath{}, fmt.Errorf("unexpected segment path")
	}
	if parts[segmentsIndex+1] != "o" {
		return timeshiftSegmentPath{}, fmt.Errorf("non-program segment class %q", parts[segmentsIndex+1])
	}
	if len(parts) != segmentsIndex+6 {
		return timeshiftSegmentPath{}, fmt.Errorf("unexpected segment path")
	}

	return timeshiftSegmentPath{
		Path:      u.Path,
		StationID: parts[segmentsIndex+3],
		Date:      parts[segmentsIndex+4],
		Filename:  parts[segmentsIndex+5],
	}, nil
}

func parseTimeshiftProgramSegment(
	raw string,
	stationID string,
	from time.Time,
	to time.Time,
	location *time.Location,
) (timeshiftProgramSegment, error) {
	segmentPath, err := parseTimeshiftSegmentPath(raw)
	if err != nil {
		return timeshiftProgramSegment{}, err
	}
	if segmentPath.StationID != stationID {
		return timeshiftProgramSegment{}, fmt.Errorf(
			"station mismatch: got %q want %q",
			segmentPath.StationID,
			stationID,
		)
	}

	filename := segmentPath.Filename
	const minimumProgramSegmentFilenameLength = len("20060102_150405_x.aac")
	if len(filename) < minimumProgramSegmentFilenameLength ||
		filename[8] != '_' ||
		filename[15] != '_' ||
		!strings.HasSuffix(filename, ".aac") {
		return timeshiftProgramSegment{}, fmt.Errorf("unexpected program segment filename")
	}

	timestampValue := filename[:8] + filename[9:15]
	if segmentPath.Date != timestampValue[:8] {
		return timeshiftProgramSegment{}, fmt.Errorf(
			"date mismatch: path=%q filename=%q",
			segmentPath.Date,
			timestampValue[:8],
		)
	}
	timestamp, err := time.ParseInLocation(DatetimeLayout, timestampValue, location)
	if err != nil {
		return timeshiftProgramSegment{}, fmt.Errorf("parse segment timestamp: %w", err)
	}
	if timestamp.Before(from) || !timestamp.Before(to) {
		return timeshiftProgramSegment{}, fmt.Errorf(
			"timestamp outside program range: %s",
			timestampValue,
		)
	}

	return timeshiftProgramSegment{
		Path:      segmentPath.Path,
		StationID: segmentPath.StationID,
		Date:      segmentPath.Date,
		Timestamp: timestamp,
	}, nil
}

func timeshiftDebugEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(timeshiftDebugEnv))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func dumpChunklist(path string, chunks []string) (err error) {
	// #nosec G703 -- developer explicitly selects diagnostic dump destination.
	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		timeshiftDiagnosticPerm,
	)
	if err != nil {
		return fmt.Errorf("create dump file: %w", err)
	}
	// When truncate, the existing permission remains; chmod to overwrite
	if err := file.Chmod(timeshiftDiagnosticPerm); err != nil {
		return fmt.Errorf("chmod dump file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close dump file: %w", closeErr))
		}
	}()

	writer := bufio.NewWriter(file)
	for i, chunk := range chunks {
		if _, err := writer.WriteString(chunk + "\n"); err != nil {
			return fmt.Errorf("write chunk %d: %w", i, err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush dump file: %w", err)
	}
	return nil
}
