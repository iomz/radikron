package radikron

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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

func Download(
	ctx context.Context,
	wg *sync.WaitGroup,
	prog *Prog,
) (err error) {
	asset := GetAsset(ctx)
	title := prog.Title
	start := prog.Ft
	var startTime, nextEndTime time.Time

	startTime, err = time.ParseInLocation(DatetimeLayout, start, Location)
	if err != nil {
		return fmt.Errorf("invalid start time format '%s': %w", start, err)
	}

	// the program is in the future
	if startTime.After(CurrentTime) {
		nextEndTime, err = time.ParseInLocation(DatetimeLayout, prog.To, Location)
		if err != nil {
			return fmt.Errorf("invalid end time format '%s': %w", prog.To, err)
		}
		// update the next fetching time
		if asset.NextFetchTime == nil || asset.NextFetchTime.After(nextEndTime) {
			next := nextEndTime.Add(BufferMinutes * time.Minute)
			asset.NextFetchTime = &next
		}
		return nil
	}

	// the program is already to be downloaded
	// Note: Duplicate check and schedule addition is now done in the monitoring loop before calling Download()
	// This check remains as a safety net in case Download() is called directly
	if asset.Schedules.HasDuplicate(prog) {
		log.Printf("-skip duplicate [%s]%s (%s)", prog.StationID, title, start)
		return nil
	}
	// Add to schedules if not already added (safety net for direct calls)
	// In the GUI monitoring loop, this is already done before calling Download()
	if !asset.Schedules.HasDuplicate(prog) {
		asset.Schedules = append(asset.Schedules, prog)
	}

	// the output config
	fileBaseName := fmt.Sprintf(
		"%s_%s_%s",
		startTime.In(Location).Format(OutputDatetimeLayout),
		prog.StationID,
		title,
	)
	output, err := newOutputConfig(
		fileBaseName,
		asset.OutputFormat,
		asset.DownloadDir,
		prog.RuleFolder,
	)
	if err != nil {
		return fmt.Errorf("failed to configure output: %w", err)
	}
	if err = output.SetupDir(); err != nil {
		return fmt.Errorf("failed to setup the output dir: %w", err)
	}

	// Check for duplicates and move from default folder to configured folder if needed
	// handleDuplicate checks the target location first and handles skip cases
	if err := handleDuplicate(fileBaseName, asset.OutputFormat, asset.DownloadDir, prog.RuleFolder, output, asset.Rules); err != nil {
		// If errSkipAfterMove, file was moved and exists at target - skip without logging again
		if errors.Is(err, errSkipAfterMove) {
			return nil
		}
		return fmt.Errorf("failed to handle duplicate: %w", err)
	}

	// Final check: verify target location doesn't exist before proceeding with download
	// This is necessary because handleDuplicate may find duplicates in other folders and return early,
	// so we need to verify the target location specifically to prevent downloading when it already exists.
	if output.IsExist() {
		log.Printf("-skip already exists: %s", output.AbsPath())
		return nil
	}

	// fetch the recording m3u8 uri
	uri, err := timeshiftProgM3U8(ctx, prog)
	if err != nil {
		return fmt.Errorf(
			"playlist.m3u8 not available [%s]%s (%s): %s",
			prog.StationID,
			title,
			start,
			err,
		)
	}
	// Log rule match only when download actually starts (not skipped)
	if prog.RuleName != "" {
		log.Printf("rule[%s] matched: [%s]%s (%s)", prog.RuleName, prog.StationID, title, start)
	}
	log.Printf("start downloading [%s]%s (%s): %s", prog.StationID, title, start, uri)
	prog.M3U8 = uri
	wg.Add(1)
	go downloadProgram(ctx, wg, prog, output)
	return nil
}

func buildM3U8RequestURI(prog *Prog) string {
	u, err := url.Parse(APIPlaylistM3U8)
	if err != nil {
		log.Fatal(err)
	}
	// set query parameters
	urlQuery := u.Query()
	params := map[string]string{
		"station_id": prog.StationID,
		"ft":         prog.Ft,
		"to":         prog.To,
		"l":          PlaylistM3U8Length, // required?
	}
	for k, v := range params {
		urlQuery.Set(k, v)
	}
	u.RawQuery = urlQuery.Encode()

	return u.String()
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
			for i := 0; i < MaxRetryAttempts; i++ {
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

	chunklist, err := getChunklistFromM3U8(prog.M3U8)
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
		log.Printf("ID3v2: %v", err)
		return
	}

	// finish downloading the file
	log.Printf("+file saved: %s", output.AbsPath())
}

// writeOutputFile writes the concatenated file to the output location,
// handling format conversion (AAC to MP3) if needed.
func writeOutputFile(ctx context.Context, concatedFile string, output *radigo.OutputConfig) error {
	switch output.AudioFormat() {
	case radigo.AudioFormatAAC:
		return os.Rename(concatedFile, output.AbsPath())
	case radigo.AudioFormatMP3:
		// Limit concurrent encoding operations to prevent resource exhaustion
		encodingSem <- struct{}{}
		defer func() { <-encodingSem }()
		log.Printf("start encoding to MP3: %s", output.AbsPath())
		return convertAACtoMP3(ctx, concatedFile, output.AbsPath())
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

// getChunklist returns a slice of uri string.
func getChunklist(input io.Reader) ([]string, error) {
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

// getChunklistFromM3U8 returns a slice of url.
func getChunklistFromM3U8(uri string) ([]string, error) {
	resp, err := http.Get(uri) //nolint:gosec,noctx
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return getChunklist(resp.Body)
}

// getRadicronPath gets the RADICRON_HOME path
func getRadicronPath(sub string) (string, error) {
	// If the environment variable RADICRON_HOME is set,
	// override working directory path.
	fullPath := os.Getenv(EnvRadicronHome)
	switch {
	case fullPath != "" && !filepath.IsAbs(fullPath):
		// Relative path - need working directory
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		fullPath = filepath.Join(wd, fullPath, sub)
	case fullPath == "":
		// Default path - need working directory
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		fullPath = filepath.Join(wd, "radiko", sub)
	default:
		// Absolute path - no need for working directory
		fullPath = filepath.Join(fullPath, sub)
	}
	return filepath.Clean(fullPath), nil
}

// getURI returns uri generated by parsing m3u8.
func getURI(input io.Reader) (string, error) {
	playlist, listType, err := m3u8.DecodeFrom(input, true)
	if err != nil || listType != m3u8.MASTER {
		return "", err
	}
	p := playlist.(*m3u8.MasterPlaylist)

	if p == nil || len(p.Variants) != 1 || p.Variants[0] == nil {
		return "", errors.New("invalid m3u8 format")
	}
	return p.Variants[0].URI, nil
}

// newOutputConfigFromPath creates an OutputConfig from a directory path, file base name, and format.
func newOutputConfigFromPath(dirPath, fileBaseName, fileFormat string) *radigo.OutputConfig {
	return &radigo.OutputConfig{
		DirFullPath:  dirPath,
		FileBaseName: fileBaseName,
		FileFormat:   fileFormat,
	}
}

// checkDuplicate checks if a file exists in either the default download directory
// or in the configured folder. Returns true and the path if found, false otherwise.
func checkDuplicate(fileBaseName, fileFormat, downloadDir, configuredFolder string) (exists bool, existingPath string) {
	// Check in default download directory
	defaultPath, err := getRadicronPath(downloadDir)
	if err == nil {
		defaultOutput := newOutputConfigFromPath(defaultPath, fileBaseName, fileFormat)
		if defaultOutput.IsExist() {
			return true, defaultOutput.AbsPath()
		}
	}

	// Check in configured folder if different from default
	if configuredFolder != "" {
		configuredPath, err := getRadicronPath(filepath.Join(downloadDir, configuredFolder))
		if err == nil {
			configuredOutput := newOutputConfigFromPath(configuredPath, fileBaseName, fileFormat)
			if configuredOutput.IsExist() {
				return true, configuredOutput.AbsPath()
			}
		}
	}

	return false, ""
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

	// Delete source file after successful copy
	if err := os.Remove(source); err != nil {
		// Clean up destination file if delete failed (atomic operation)
		_ = os.Remove(dest)
		return fmt.Errorf("failed to remove source file after copy: %w", err)
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
		configuredPath, err := getRadicronPath(filepath.Join(downloadDir, folder))
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
func handleMoveFromDefaultFolder(source, targetPath string, output *radigo.OutputConfig) error {
	// Check if target already exists (edge case: file appeared between checks or race condition)
	if output.IsExist() {
		log.Printf("-skip target already exists, removing source: %s (target: %s)", source, targetPath)
		// Target exists, remove source file to avoid duplicates
		if err := os.Remove(source); err != nil {
			log.Printf("warning: failed to remove source file %s after target exists: %v", source, err)
		}
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
func handleDuplicate(fileBaseName, fileFormat, downloadDir, configuredFolder string, output *radigo.OutputConfig, rules Rules) error {
	// Check target location first - if file already exists at target, skip immediately
	if output.IsExist() {
		log.Printf("-skip already exists: %s", output.AbsPath())
		return nil
	}

	// Collect all unique configured folders from all rules
	configuredFolders := collectConfiguredFolders(configuredFolder, rules)

	// Check in all configured folders (excluding target, which we already checked)
	// This takes precedence over default folder
	targetPath := output.AbsPath()
	exists, existingPath := checkConfiguredFoldersForDuplicate(
		configuredFolders, downloadDir, fileBaseName, fileFormat, targetPath)
	if exists {
		log.Printf("-skip already exists: %s", existingPath)
		return nil
	}

	// Check in default download directory
	defaultPath, err := getRadicronPath(downloadDir)
	if err != nil {
		return nil
	}
	defaultOutput := newOutputConfigFromPath(defaultPath, fileBaseName, fileFormat)
	if !defaultOutput.IsExist() {
		return nil
	}

	// If file exists in default folder and there's a configured folder, move it
	if configuredFolder != "" {
		return handleMoveFromDefaultFolder(defaultOutput.AbsPath(), output.AbsPath(), output)
	}

	// File exists in default folder, no configured folder - skip
	log.Printf("-skip already exists: %s", defaultOutput.AbsPath())
	return nil
}

// newOutputConfig prepares the outputdir
func newOutputConfig(fileBaseName, fileFormat, downloadDir, folder string) (*radigo.OutputConfig, error) {
	basePath := downloadDir
	if folder != "" {
		basePath = filepath.Join(downloadDir, folder)
	}
	fullPath, err := getRadicronPath(basePath)
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
	fullPath, err := getRadicronPath("tmp")
	if err != nil {
		return "", err
	}

	// Ensure the tmp directory exists
	if err := os.MkdirAll(fullPath, DirPermissions); err != nil {
		return "", err
	}

	aacDir, err := os.MkdirTemp(fullPath, "aac")
	if err != nil {
		return "", err
	}

	return aacDir, nil
}

// timeshiftProgM3U8 gets playlist.m3u8 for a Prog
func timeshiftProgM3U8(
	ctx context.Context,
	prog *Prog,
) (string, error) {
	asset := GetAsset(ctx)
	client := asset.DefaultClient
	var req *http.Request
	var err error

	areaID := asset.GetAreaIDByStationID(prog.StationID)

	device, ok := asset.AreaDevices[areaID]
	if !ok {
		device, err = asset.NewDevice(ctx, areaID)
		if err != nil {
			return "", err
		}
	}

	uri := buildM3U8RequestURI(prog)
	req, err = http.NewRequestWithContext(ctx, "POST", uri, http.NoBody)
	if err != nil {
		return "", err
	}
	headers := map[string]string{
		UserAgentHeader:       device.UserAgent,
		RadikoAreaIDHeader:    areaID,
		RadikoAuthTokenHeader: device.AuthToken,
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return getURI(resp.Body)
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
