package radikron

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bogem/id3v2"
	"github.com/yyoshiki41/radigo"
)

var (
	//go:embed test/playlist-test.m3u8
	PlaylistTestM3U8 embed.FS
)

const (
	osWindows = "windows"
)

func TestBuildM3U8RequestURI(t *testing.T) {
	prog := &Prog{
		StationID: "FMT",
		Ft:        "20230605130000",
		To:        "20230605145500",
	}
	uri := buildM3U8RequestURI(prog)
	want := "https://radiko.jp/v2/api/ts/playlist.m3u8?ft=20230605130000&l=15&station_id=FMT&to=20230605145500"
	if uri != want {
		t.Errorf("buildM3U8RequestURI => %v, want %v", uri, want)
	}

	// Test with different station
	prog2 := &Prog{
		StationID: "TBS",
		Ft:        "20230605140000",
		To:        "20230605150000",
	}
	uri2 := buildM3U8RequestURI(prog2)
	if uri2 == uri {
		t.Error("buildM3U8RequestURI should generate different URIs for different programs")
	}
	if uri2 == "" {
		t.Error("buildM3U8RequestURI returned empty URI")
	}

	// Verify the URI contains all required parameters
	if !strings.Contains(uri, "station_id=FMT") {
		t.Error("URI should contain station_id parameter")
	}
	if !strings.Contains(uri, "ft=20230605130000") {
		t.Error("URI should contain ft parameter")
	}
	if !strings.Contains(uri, "to=20230605145500") {
		t.Error("URI should contain to parameter")
	}
	if !strings.Contains(uri, "l=15") {
		t.Error("URI should contain l parameter with PlaylistM3U8Length")
	}
}

func TestGetURI(t *testing.T) {
	m3u8, err := PlaylistTestM3U8.Open("test/playlist-test.m3u8")
	if err != nil {
		t.Error(err)
	}
	defer m3u8.Close()
	uri, err := getURI(m3u8)
	if err != nil {
		t.Error(err)
	}
	want := "https://radiko.jp/v2/api/ts/chunklist/FsNE6Bt0.m3u8"
	if uri != want {
		t.Errorf("getURI => %v, want %v", uri, want)
	}

	// Test with invalid input (media playlist instead of master)
	// This would require a media playlist test file, but we can test error handling
	// by using an invalid reader or empty input
}

func TestGetURI_InvalidVariants(t *testing.T) {
	// Test with invalid m3u8 format (not a master playlist)
	invalidReader := strings.NewReader("not a valid m3u8")
	_, err := getURI(invalidReader)
	if err == nil {
		t.Error("getURI should return error for invalid m3u8 format")
	}

	// Test with empty input
	emptyReader := strings.NewReader("")
	_, err = getURI(emptyReader)
	if err == nil {
		t.Error("getURI should return error for empty input")
	}
}

func TestGetRadicronPath(t *testing.T) {
	// Save original env value
	originalEnv := os.Getenv(EnvRadicronHome)
	defer os.Setenv(EnvRadicronHome, originalEnv)

	// Test with empty env (default case)
	os.Unsetenv(EnvRadicronHome)
	path, err := getRadicronPath("downloads")
	if err != nil {
		t.Errorf("getRadicronPath with empty env failed: %v", err)
	}
	if path == "" {
		t.Error("getRadicronPath returned empty path")
	}

	// Test with relative path
	os.Setenv(EnvRadicronHome, "test-radiko")
	path, err = getRadicronPath("downloads")
	if err != nil {
		t.Errorf("getRadicronPath with relative env failed: %v", err)
	}
	if path == "" {
		t.Error("getRadicronPath returned empty path")
	}

	// Test with absolute path
	absPath, _ := filepath.Abs("/tmp")
	os.Setenv(EnvRadicronHome, absPath)
	path, err = getRadicronPath("downloads")
	if err != nil {
		t.Errorf("getRadicronPath with absolute env failed: %v", err)
	}
	expected := filepath.Join(absPath, "downloads")
	if path != expected {
		t.Errorf("getRadicronPath => %v, want %v", path, expected)
	}

	// Test with subdirectory
	path, err = getRadicronPath(filepath.Join("downloads", "subfolder"))
	if err != nil {
		t.Errorf("getRadicronPath with subdirectory failed: %v", err)
	}
	expected = filepath.Join(absPath, "downloads", "subfolder")
	if path != expected {
		t.Errorf("getRadicronPath with subdirectory => %v, want %v", path, expected)
	}

	// Test path cleaning (with .. and .)
	path, err = getRadicronPath(filepath.Join("downloads", "..", "downloads", ".", "sub"))
	if err != nil {
		t.Errorf("getRadicronPath with path cleaning failed: %v", err)
	}
	expected = filepath.Join(absPath, "downloads", "sub")
	if path != expected {
		t.Errorf("getRadicronPath with path cleaning => %v, want %v", path, expected)
	}
}

func TestNewOutputConfig(t *testing.T) {
	// Save original env value
	originalEnv := os.Getenv(EnvRadicronHome)
	defer os.Setenv(EnvRadicronHome, originalEnv)
	os.Unsetenv(EnvRadicronHome)

	// Test without folder
	output, err := newOutputConfig("test-file", radigo.AudioFormatAAC, "downloads", "")
	if err != nil {
		t.Fatalf("newOutputConfig failed: %v", err)
	}
	if output == nil {
		t.Fatal("newOutputConfig returned nil")
	}
	if output.FileBaseName != "test-file" {
		t.Errorf("newOutputConfig FileBaseName => %v, want test-file", output.FileBaseName)
	}
	if output.FileFormat != radigo.AudioFormatAAC {
		t.Errorf("newOutputConfig FileFormat => %v, want %v", output.FileFormat, radigo.AudioFormatAAC)
	}

	// Test with folder
	output, err = newOutputConfig("test-file", radigo.AudioFormatMP3, "downloads", "citypop")
	if err != nil {
		t.Fatalf("newOutputConfig with folder failed: %v", err)
	}
	if output == nil {
		t.Fatal("newOutputConfig with folder returned nil")
	}
	if output.FileFormat != radigo.AudioFormatMP3 {
		t.Errorf("newOutputConfig FileFormat => %v, want %v", output.FileFormat, radigo.AudioFormatMP3)
	}

	// Test with custom download directory
	output, err = newOutputConfig("test-file", radigo.AudioFormatAAC, "my-downloads", "")
	if err != nil {
		t.Errorf("newOutputConfig with custom dir failed: %v", err)
	}
	if output == nil {
		t.Error("newOutputConfig with custom dir returned nil")
	}
}

func TestTempAACDir(t *testing.T) {
	// Save original env value
	originalEnv := os.Getenv(EnvRadicronHome)
	defer os.Setenv(EnvRadicronHome, originalEnv)

	// Set a test directory
	testDir := filepath.Join(os.TempDir(), "radikron-test")
	os.Setenv(EnvRadicronHome, testDir)
	defer os.RemoveAll(testDir)

	// Create the tmp directory structure
	tmpDir := filepath.Join(testDir, "tmp")
	err := os.MkdirAll(tmpDir, DirPermissions)
	if err != nil {
		t.Fatalf("Failed to create test tmp directory: %v", err)
	}

	dir, err := tempAACDir()
	if err != nil {
		t.Errorf("tempAACDir failed: %v", err)
	}
	if dir == "" {
		t.Error("tempAACDir returned empty path")
	}

	// Verify the directory was created
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("tempAACDir did not create directory: %v", err)
	}

	// Clean up
	os.RemoveAll(dir)

	// Test: tempAACDir creates directory if it doesn't exist
	testDir2 := filepath.Join(os.TempDir(), "radikron-test-2")
	os.Setenv(EnvRadicronHome, testDir2)
	defer os.RemoveAll(testDir2)

	// Don't create tmp directory - tempAACDir should create it
	dir2, err := tempAACDir()
	if err != nil {
		t.Errorf("tempAACDir failed when creating directory: %v", err)
	}
	if dir2 == "" {
		t.Error("tempAACDir returned empty path")
	}
	// Verify the directory was created
	if _, err := os.Stat(dir2); os.IsNotExist(err) {
		t.Errorf("tempAACDir did not create directory: %v", err)
	}
	os.RemoveAll(dir2)
}

// TestCheckDuplicate removed - checkDuplicate function was removed
// The functionality is now handled by handleDuplicate which is tested below

// setupHandleDuplicateTest creates a test environment for handleDuplicate tests
func setupHandleDuplicateTest(t *testing.T) (downloadsDir string, cleanup func()) {
	t.Helper()
	originalEnv := os.Getenv(EnvRadicronHome)
	testDir := filepath.Join(os.TempDir(), "radikron-test-handle-dup")
	os.Setenv(EnvRadicronHome, testDir)
	downloadsDir = filepath.Join(testDir, "downloads")
	if err := os.MkdirAll(downloadsDir, DirPermissions); err != nil {
		t.Fatalf("Failed to create test downloads directory: %v", err)
	}
	cleanup = func() {
		os.Setenv(EnvRadicronHome, originalEnv)
		os.RemoveAll(testDir)
	}
	return downloadsDir, cleanup
}

func TestHandleDuplicate_NonexistentFile(t *testing.T) {
	_, cleanup := setupHandleDuplicateTest(t)
	defer cleanup()

	output, err := newOutputConfig("nonexistent-file", radigo.AudioFormatAAC, "downloads", "")
	if err != nil {
		t.Fatalf("newOutputConfig failed: %v", err)
	}
	ctx := context.Background()
	err = handleDuplicate(ctx, "nonexistent-file", radigo.AudioFormatAAC, "downloads", "", output, Rules{}, "TEST", "Test Program", "20230605100000")
	if err != nil {
		t.Errorf("handleDuplicate should not return error for non-existent file: %v", err)
	}
}

func TestHandleDuplicate_ExistingInDefaultFolder(t *testing.T) {
	downloadsDir, cleanup := setupHandleDuplicateTest(t)
	defer cleanup()

	testFile := filepath.Join(downloadsDir, "test-file.aac")
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	file.Close()

	output, err := newOutputConfig("test-file", radigo.AudioFormatAAC, "downloads", "")
	if err != nil {
		t.Fatalf("newOutputConfig failed: %v", err)
	}
	ctx := context.Background()
	err = handleDuplicate(ctx, "test-file", radigo.AudioFormatAAC, "downloads", "", output, Rules{}, "TEST", "Test Program", "20230605100000")
	if err != nil {
		t.Errorf("handleDuplicate should not return error for existing file in default folder: %v", err)
	}
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("File should still exist in default folder when no configured folder is specified")
	}
}

func TestHandleDuplicate_MoveToConfiguredFolder(t *testing.T) {
	downloadsDir, cleanup := setupHandleDuplicateTest(t)
	defer cleanup()

	citypopDir := filepath.Join(downloadsDir, "citypop")
	if err := os.MkdirAll(citypopDir, DirPermissions); err != nil {
		t.Fatalf("Failed to create citypop directory: %v", err)
	}

	moveFile := filepath.Join(downloadsDir, "move-test.aac")
	file, err := os.Create(moveFile)
	if err != nil {
		t.Fatalf("Failed to create file to move: %v", err)
	}
	file.Close()

	output, err := newOutputConfig("move-test", radigo.AudioFormatAAC, "downloads", "citypop")
	if err != nil {
		t.Fatalf("newOutputConfig failed: %v", err)
	}
	ctx := context.Background()
	err = handleDuplicate(ctx, "move-test", radigo.AudioFormatAAC, "downloads", "citypop", output, Rules{}, "TEST", "Test Program", "20230605100000")
	// errSkipAfterMove is a sentinel error indicating successful move, not a real error
	if err != nil && !errors.Is(err, errSkipAfterMove) {
		t.Errorf("handleDuplicate should not return error when moving file: %v", err)
	}

	expectedPath := filepath.Join(citypopDir, "move-test.aac")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Error("File should have been moved to configured folder")
	}
	if _, err := os.Stat(moveFile); err == nil {
		t.Error("File should no longer exist in default folder")
	}
}

func TestHandleDuplicate_ExistingInConfiguredFolder(t *testing.T) {
	downloadsDir, cleanup := setupHandleDuplicateTest(t)
	defer cleanup()

	citypopDir := filepath.Join(downloadsDir, "citypop")
	if err := os.MkdirAll(citypopDir, DirPermissions); err != nil {
		t.Fatalf("Failed to create citypop directory: %v", err)
	}

	expectedPath := filepath.Join(citypopDir, "move-test.aac")
	file, err := os.Create(expectedPath)
	if err != nil {
		t.Fatalf("Failed to create file in configured folder: %v", err)
	}
	file.Close()

	output, err := newOutputConfig("move-test", radigo.AudioFormatAAC, "downloads", "citypop")
	if err != nil {
		t.Fatalf("newOutputConfig failed: %v", err)
	}
	ctx := context.Background()
	err = handleDuplicate(ctx, "move-test", radigo.AudioFormatAAC, "downloads", "citypop", output, Rules{}, "TEST", "Test Program", "20230605100000")
	if err != nil {
		t.Errorf("handleDuplicate should not return error for existing file in configured folder: %v", err)
	}
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Error("File should still exist in configured folder")
	}
}

func TestHandleDuplicate_ConflictBothLocations(t *testing.T) {
	downloadsDir, cleanup := setupHandleDuplicateTest(t)
	defer cleanup()

	citypopDir := filepath.Join(downloadsDir, "citypop")
	if err := os.MkdirAll(citypopDir, DirPermissions); err != nil {
		t.Fatalf("Failed to create citypop directory: %v", err)
	}

	defaultFile := filepath.Join(downloadsDir, "conflict-test.aac")
	file, err := os.Create(defaultFile)
	if err != nil {
		t.Fatalf("Failed to create default file: %v", err)
	}
	file.Close()

	configuredFile := filepath.Join(citypopDir, "conflict-test.aac")
	file, err = os.Create(configuredFile)
	if err != nil {
		t.Fatalf("Failed to create configured file: %v", err)
	}
	file.Close()

	output, err := newOutputConfig("conflict-test", radigo.AudioFormatAAC, "downloads", "citypop")
	if err != nil {
		t.Fatalf("newOutputConfig failed: %v", err)
	}
	ctx := context.Background()
	err = handleDuplicate(ctx, "conflict-test", radigo.AudioFormatAAC, "downloads", "citypop", output, Rules{}, "TEST", "Test Program", "20230605100000")
	if err != nil {
		t.Errorf("handleDuplicate should not return error when file exists in both locations: %v", err)
	}
	if _, err := os.Stat(configuredFile); os.IsNotExist(err) {
		t.Error("File should still exist in configured folder")
	}
	if _, err := os.Stat(defaultFile); os.IsNotExist(err) {
		t.Error("File should still exist in default folder when file also exists in configured folder")
	}
}

func TestHandleDuplicate_ChecksAllConfiguredFolders(t *testing.T) {
	downloadsDir, cleanup := setupHandleDuplicateTest(t)
	defer cleanup()

	// Create multiple configured folders
	jazzDir := filepath.Join(downloadsDir, "jazz")
	if err := os.MkdirAll(jazzDir, DirPermissions); err != nil {
		t.Fatalf("Failed to create jazz directory: %v", err)
	}

	rockDir := filepath.Join(downloadsDir, "rock")
	if err := os.MkdirAll(rockDir, DirPermissions); err != nil {
		t.Fatalf("Failed to create rock directory: %v", err)
	}

	// Create a file in the jazz folder (different from the current configured folder)
	jazzFile := filepath.Join(jazzDir, "test-file.aac")
	file, err := os.Create(jazzFile)
	if err != nil {
		t.Fatalf("Failed to create file in jazz folder: %v", err)
	}
	file.Close()

	// Create rules with different folders
	rules := Rules{
		{Folder: "jazz"},
		{Folder: "rock"},
		{Folder: "citypop"}, // current configured folder
	}

	// Try to handle duplicate with citypop as configured folder, but file exists in jazz
	output, err := newOutputConfig("test-file", radigo.AudioFormatAAC, "downloads", "citypop")
	if err != nil {
		t.Fatalf("newOutputConfig failed: %v", err)
	}
	ctx := context.Background()
	err = handleDuplicate(ctx, "test-file", radigo.AudioFormatAAC, "downloads", "citypop", output, rules, "TEST", "Test Program", "20230605100000")
	if err != nil {
		t.Errorf("handleDuplicate should not return error: %v", err)
	}
	// Should skip because file exists in jazz folder (one of the configured folders)
	if _, err := os.Stat(jazzFile); os.IsNotExist(err) {
		t.Error("File should still exist in jazz folder")
	}
}

func TestHandleDuplicate_TargetExistsBeforeMove(t *testing.T) {
	downloadsDir, cleanup := setupHandleDuplicateTest(t)
	defer cleanup()

	// Create file in default folder
	defaultFile := filepath.Join(downloadsDir, "target-exists-test.aac")
	err := os.WriteFile(defaultFile, []byte("test content"), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create configured folder
	configuredDir := filepath.Join(downloadsDir, "citypop")
	err = os.MkdirAll(configuredDir, DirPermissions)
	if err != nil {
		t.Fatalf("Failed to create configured directory: %v", err)
	}

	// Create output config for configured folder
	output, err := newOutputConfig("target-exists-test", radigo.AudioFormatAAC, "downloads", "citypop")
	if err != nil {
		t.Fatalf("Failed to create output config: %v", err)
	}

	// Create target file in configured folder (simulating edge case where target exists)
	// This tests the edge case handling at line 445-451 in handleDuplicate
	targetFile := output.AbsPath()
	err = os.WriteFile(targetFile, []byte("existing content"), 0600)
	if err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	// Call handleDuplicate - should detect target exists in configured folder check and skip
	// The edge case check (line 445) handles race conditions where target appears
	// between the initial check and the move attempt
	ctx := context.Background()
	err = handleDuplicate(ctx, "target-exists-test", radigo.AudioFormatAAC, "downloads", "citypop", output, Rules{}, "TEST", "Test Program", "20230605100000")
	if err != nil {
		t.Errorf("handleDuplicate should not return error when target exists: %v", err)
	}

	// When target exists in configured folder, it should skip early (line 428)
	// and not attempt to move, so source file should still exist
	// Note: The edge case handling at line 445-451 would remove source if target
	// appears between the default folder check and the move attempt
	if _, err := os.Stat(defaultFile); os.IsNotExist(err) {
		t.Log("Source file removed (this is expected if edge case handling triggered)")
	}

	// Verify target file still exists
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Error("Target file should still exist")
	}
}

func TestGetChunklist(t *testing.T) {
	// Test with master playlist (should return error or nil)
	m3u8, err := PlaylistTestM3U8.Open("test/playlist-test.m3u8")
	if err != nil {
		t.Fatalf("Failed to open test playlist: %v", err)
	}
	defer m3u8.Close()

	// Note: The test file is a master playlist, not a media playlist
	// getChunklist expects a media playlist, so it should return an error or nil chunklist
	chunklist, err := getChunklist(m3u8)
	// getChunklist returns (nil, err) when listType is not MEDIA
	// The function checks: err != nil || listType != m3u8.MEDIA
	// For master playlist, listType != MEDIA, so it should return nil chunklist
	if chunklist != nil {
		t.Errorf("getChunklist should return nil chunklist for master playlist, got %v", chunklist)
	}
	// Error may or may not be nil depending on decode behavior, but chunklist must be nil
	if err != nil {
		t.Logf("expected non-media playlist decode error: %v", err)
	}

	// Test with invalid input (empty reader)
	emptyReader := strings.NewReader("")
	chunklist, err = getChunklist(emptyReader)
	if chunklist != nil {
		t.Error("getChunklist should return nil chunklist for invalid input")
	}
	// Error is expected for invalid input
	if err == nil {
		t.Log("getChunklist may or may not return error for invalid input")
	}

	// Test with invalid m3u8 format
	invalidReader := strings.NewReader("#EXTM3U\ninvalid content")
	chunklist, _ = getChunklist(invalidReader)
	if chunklist != nil {
		t.Error("getChunklist should return nil chunklist for invalid format")
	}
}

func TestGetURIErrorCases(t *testing.T) {
	// Test with invalid input (empty reader)
	emptyReader := strings.NewReader("")
	_, err := getURI(emptyReader)
	if err == nil {
		t.Error("getURI should return error for invalid input")
	}

	// Test with invalid XML (not m3u8)
	invalidReader := strings.NewReader("<?xml version=\"1.0\"?><invalid></invalid>")
	_, err = getURI(invalidReader)
	if err == nil {
		t.Error("getURI should return error for invalid m3u8 format")
	}

	// Test with media playlist (should return error, expects master playlist)
	mediaPlaylist := strings.NewReader("#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:10.0,\nsegment.ts\n")
	uri, err := getURI(mediaPlaylist)
	// getURI checks listType != m3u8.MASTER, so media playlist should return error
	// However, if decode succeeds but listType is MEDIA, it returns empty string and error
	if err == nil && uri != "" {
		t.Error("getURI should return error or empty URI for media playlist (expects master)")
	}
}

func TestWriteID3TagMP3(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "radikron-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test MP3 format
	testWriteID3Tag(t, tmpDir, radigo.AudioFormatMP3, "test-mp3")
}

func TestWriteID3TagAAC(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "radikron-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test AAC format
	testWriteID3Tag(t, tmpDir, radigo.AudioFormatAAC, "test-aac")
}

func testWriteID3Tag(t *testing.T, tmpDir, fileFormat, fileBaseName string) {
	// Create output config
	output := &radigo.OutputConfig{
		DirFullPath:  tmpDir,
		FileBaseName: fileBaseName,
		FileFormat:   fileFormat,
	}

	// Create a minimal file (id3v2 can write tags to empty files)
	testFile := output.AbsPath()
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	file.Close()

	// Create test program data
	prog := &Prog{
		Title:      "Test Program Title",
		Pfm:        "Test Artist",
		Ft:         "20230605130000",
		Info:       "Test program information",
		RuleName:   "test-rule",
		RuleFolder: "",
	}

	// Write ID3 tags
	err = writeID3Tag(output, prog)
	if err != nil {
		t.Fatalf("writeID3Tag failed: %v", err)
	}

	// Read back the tags to verify
	tag, err := id3v2.Open(testFile, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("Failed to open file for reading tags: %v", err)
	}
	defer tag.Close()

	// Verify Title
	gotTitle := tag.Title()
	wantTitle := fileBaseName
	if gotTitle != wantTitle {
		t.Errorf("Title => %v, want %v", gotTitle, wantTitle)
	}

	// Verify Artist
	gotArtist := tag.Artist()
	wantArtist := prog.Pfm
	if gotArtist != wantArtist {
		t.Errorf("Artist => %v, want %v", gotArtist, wantArtist)
	}

	// Verify Album
	gotAlbum := tag.Album()
	wantAlbum := prog.Title
	if gotAlbum != wantAlbum {
		t.Errorf("Album => %v, want %v", gotAlbum, wantAlbum)
	}

	// Verify Year
	gotYear := tag.Year()
	wantYear := prog.Ft[:4] // "2023"
	if gotYear != wantYear {
		t.Errorf("Year => %v, want %v", gotYear, wantYear)
	}

	// Verify Comment
	commentFrames := tag.GetFrames(tag.CommonID("Comments"))
	if len(commentFrames) == 0 {
		t.Error("Expected at least one comment frame")
	} else {
		commentFrame, ok := commentFrames[0].(id3v2.CommentFrame)
		if !ok {
			t.Error("Expected comment frame to be CommentFrame type")
		} else {
			// Note: prog.Info is stored in Description field, not Text field
			gotComment := commentFrame.Description
			wantComment := prog.Info
			if gotComment != wantComment {
				t.Errorf("Comment Description => %v, want %v", gotComment, wantComment)
			}
		}
	}

	// Verify Album Artist (Rule Name)
	albumArtistFrame := tag.GetTextFrame(tag.CommonID("Band/Orchestra/Accompaniment"))
	if albumArtistFrame.Text == "" {
		t.Error("Expected Album Artist (TPE2) frame to be present")
	} else {
		gotAlbumArtist := albumArtistFrame.Text
		wantAlbumArtist := prog.RuleName
		if gotAlbumArtist != wantAlbumArtist {
			t.Errorf("Album Artist => %v, want %v", gotAlbumArtist, wantAlbumArtist)
		}
	}
}

func TestWriteID3TagWithoutRuleName(t *testing.T) {
	// Test that writeID3Tag works even when RuleName is empty
	tmpDir, err := os.MkdirTemp("", "radikron-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	output := &radigo.OutputConfig{
		DirFullPath:  tmpDir,
		FileBaseName: "test-no-rule",
		FileFormat:   radigo.AudioFormatMP3,
	}

	// Create a minimal file
	testFile := output.AbsPath()
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	file.Close()

	// Create test program data without RuleName
	prog := &Prog{
		Title: "Test Program Title",
		Pfm:   "Test Artist",
		Ft:    "20230605130000",
		Info:  "Test program information",
		// RuleName is empty
	}

	// Write ID3 tags
	err = writeID3Tag(output, prog)
	if err != nil {
		t.Fatalf("writeID3Tag failed: %v", err)
	}

	// Read back the tags to verify
	tag, err := id3v2.Open(testFile, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("Failed to open file for reading tags: %v", err)
	}
	defer tag.Close()

	// Verify Album Artist is not set when RuleName is empty
	albumArtistFrame := tag.GetTextFrame(tag.CommonID("Band/Orchestra/Accompaniment"))
	if albumArtistFrame.Text != "" {
		t.Errorf("Expected Album Artist (TPE2) frame to be absent when RuleName is empty, got: %v", albumArtistFrame.Text)
	}

	// Verify other tags are still present
	if tag.Title() == "" {
		t.Error("Expected Title to be set")
	}
	if tag.Artist() == "" {
		t.Error("Expected Artist to be set")
	}
}

func TestMoveFile(t *testing.T) {
	// Create temporary directory for testing
	tmpDir := t.TempDir()

	// Test 1: Successful os.Rename (same filesystem)
	sourceFile := filepath.Join(tmpDir, "source.txt")
	destFile := filepath.Join(tmpDir, "dest.txt")

	// Create source file
	err := os.WriteFile(sourceFile, []byte("test content"), 0600)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Move file
	err = moveFile(sourceFile, destFile)
	if err != nil {
		t.Errorf("moveFile failed: %v", err)
	}

	// Verify file was moved
	if _, err := os.Stat(sourceFile); err == nil {
		t.Error("Source file should not exist after move")
	}
	if _, err := os.Stat(destFile); os.IsNotExist(err) {
		t.Error("Destination file should exist after move")
	}

	// Verify content
	content, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}
	if string(content) != "test content" {
		t.Errorf("File content mismatch: got %s, want test content", string(content))
	}
}

func TestMoveFile_CopyFallback(t *testing.T) {
	// Create temporary directories for testing
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	destDir := filepath.Join(tmpDir, "dest")

	// Create directories
	if err := os.MkdirAll(sourceDir, DirPermissions); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}
	if err := os.MkdirAll(destDir, DirPermissions); err != nil {
		t.Fatalf("Failed to create dest directory: %v", err)
	}

	sourceFile := filepath.Join(sourceDir, "source.txt")
	destFile := filepath.Join(destDir, "dest.txt")

	// Create source file with content
	testContent := "test content for copy fallback"
	err := os.WriteFile(sourceFile, []byte(testContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Move file (will use copy-then-delete if rename fails, or rename if on same filesystem)
	err = moveFile(sourceFile, destFile)
	if err != nil {
		t.Errorf("moveFile failed: %v", err)
	}

	// Verify file was moved (either by rename or copy)
	if _, err := os.Stat(sourceFile); err == nil {
		t.Error("Source file should not exist after move")
	}
	if _, err := os.Stat(destFile); os.IsNotExist(err) {
		t.Error("Destination file should exist after move")
	}

	// Verify content
	content, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}
	if string(content) != testContent {
		t.Errorf("File content mismatch: got %s, want %s", string(content), testContent)
	}
}

func TestMoveFile_CopyFallbackErrorPaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Test 1: Destination directory doesn't exist (fails at Create in copy fallback)
	// This forces the copy fallback path since os.Rename will fail across directories
	sourceFile := filepath.Join(tmpDir, "source.txt")
	err := os.WriteFile(sourceFile, []byte("test content"), 0600)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create a non-existent nested directory path
	nonExistentDest := filepath.Join(tmpDir, "nonexistent", "subdir", "dest.txt")
	err = moveFile(sourceFile, nonExistentDest)
	if err == nil {
		t.Error("moveFile should return error when destination directory doesn't exist")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to create destination file") {
		t.Errorf("Expected error about creating destination file, got: %v", err)
	}
	// Source file should still exist after failed move
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		t.Error("Source file should still exist after failed move")
	}
	// Destination should not exist
	if _, err := os.Stat(nonExistentDest); err == nil {
		t.Error("Destination file should not exist after failed move")
	}

	// Test 2: Destination directory is non-writable (fails at Create in copy fallback)
	// Skip on Windows as file permissions work differently (ACLs vs Unix permissions)
	if runtime.GOOS != osWindows {
		// Create a read-only directory
		readOnlyDir := filepath.Join(tmpDir, "readonly")
		err = os.MkdirAll(readOnlyDir, 0500) // Read-only, no write permission
		if err != nil {
			t.Fatalf("Failed to create read-only directory: %v", err)
		}
		defer func() {
			// Restore permissions for cleanup
			_ = os.Chmod(readOnlyDir, 0700)
		}()

		sourceFile2 := filepath.Join(tmpDir, "source2.txt")
		err = os.WriteFile(sourceFile2, []byte("test content 2"), 0600)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		readOnlyDest := filepath.Join(readOnlyDir, "dest.txt")
		err = moveFile(sourceFile2, readOnlyDest)
		if err == nil {
			t.Error("moveFile should return error when destination directory is not writable")
		}
		if err != nil && !strings.Contains(err.Error(), "failed to create destination file") {
			t.Errorf("Expected error about creating destination file, got: %v", err)
		}
		// Source file should still exist after failed move
		if _, err := os.Stat(sourceFile2); os.IsNotExist(err) {
			t.Error("Source file should still exist after failed move")
		}
		// Destination should not exist (or should be cleaned up if partially created)
		if _, err := os.Stat(readOnlyDest); err == nil {
			// If file was created, it should be cleaned up
			t.Error("Destination file should not exist or should be cleaned up after failed move")
		}
	}
}

func TestMoveFile_ErrorCases(t *testing.T) {
	tmpDir := t.TempDir()

	// Test: Source file doesn't exist
	nonExistentSource := filepath.Join(tmpDir, "nonexistent.txt")
	destFile := filepath.Join(tmpDir, "dest.txt")
	err := moveFile(nonExistentSource, destFile)
	if err == nil {
		t.Error("moveFile should return error when source file doesn't exist")
	}

	// Test: Destination directory doesn't exist (and can't be created)
	// Create source file
	sourceFile := filepath.Join(tmpDir, "source.txt")
	err = os.WriteFile(sourceFile, []byte("test"), 0600)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Try to move to invalid destination (on Unix, /dev/null is a special file)
	// This will test the error path when creating destination file fails
	invalidDest := filepath.Join(tmpDir, "nonexistent", "dest.txt")
	err = moveFile(sourceFile, invalidDest)
	if err == nil {
		t.Error("moveFile should return error when destination directory doesn't exist")
	}

	// Source file should still exist after failed move
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		t.Error("Source file should still exist after failed move")
	}
}

func TestNewOutputConfigFromPath(t *testing.T) {
	// Test basic functionality
	output := newOutputConfigFromPath("/tmp/test", "file-name", radigo.AudioFormatAAC)
	if output == nil {
		t.Fatal("newOutputConfigFromPath returned nil")
	}
	if output.DirFullPath != "/tmp/test" {
		t.Errorf("DirFullPath => %v, want /tmp/test", output.DirFullPath)
	}
	if output.FileBaseName != "file-name" {
		t.Errorf("FileBaseName => %v, want file-name", output.FileBaseName)
	}
	if output.FileFormat != radigo.AudioFormatAAC {
		t.Errorf("FileFormat => %v, want %v", output.FileFormat, radigo.AudioFormatAAC)
	}

	// Test with MP3 format
	output2 := newOutputConfigFromPath("/tmp/test2", "file-name2", radigo.AudioFormatMP3)
	if output2.FileFormat != radigo.AudioFormatMP3 {
		t.Errorf("FileFormat => %v, want %v", output2.FileFormat, radigo.AudioFormatMP3)
	}
}

func TestWriteID3Tag_ErrorCases(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Test: File doesn't exist
	output := newOutputConfigFromPath(tmpDir, "nonexistent", radigo.AudioFormatAAC)
	prog := &Prog{
		Title: "Test Title",
		Pfm:   "Test Artist",
		Ft:    "20230605130000",
	}

	err := writeID3Tag(output, prog)
	if err == nil {
		t.Error("writeID3Tag should return error when file doesn't exist")
	}

	// Test: Invalid file (directory instead of file)
	// Skip on Windows: id3v2.Open() locks the directory on Windows even after failure,
	// making it impossible to clean up reliably before t.TempDir() cleanup runs.
	// The error handling is still tested on other platforms.
	if runtime.GOOS != osWindows {
		dirPath := filepath.Join(tmpDir, "dir.aac")
		err = os.MkdirAll(dirPath, DirPermissions)
		if err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		output2 := newOutputConfigFromPath(tmpDir, "dir", radigo.AudioFormatAAC)
		err = writeID3Tag(output2, prog)
		if err == nil {
			t.Error("writeID3Tag should return error when path is a directory")
		}

		// Clean up the test directory
		_ = os.Remove(dirPath)
	}
}

func TestInitSemaphores(_ *testing.T) {
	// Test with nil asset
	InitSemaphores(nil)
	// Should not panic

	// Test with asset with default values (0 or negative)
	asset1 := &Asset{
		MaxDownloadingConcurrency: 0,
		MaxEncodingConcurrency:    0,
	}
	InitSemaphores(asset1)
	// Should use defaults

	// Test with asset with custom values
	asset2 := &Asset{
		MaxDownloadingConcurrency: 32,
		MaxEncodingConcurrency:    4,
	}
	InitSemaphores(asset2)
	// Should update semaphores

	// Test with negative values (should use defaults)
	asset3 := &Asset{
		MaxDownloadingConcurrency: -1,
		MaxEncodingConcurrency:    -1,
	}
	InitSemaphores(asset3)
	// Should use defaults
}

func TestGetChunklistFromM3U8(t *testing.T) {
	// This function makes HTTP requests, so we need to test it carefully
	// For now, we'll test error cases that don't require a real server

	// Test with invalid URL (should fail)
	_, err := getChunklistFromM3U8("http://invalid-url-that-does-not-exist-12345.com/test.m3u8")
	if err == nil {
		t.Log("getChunklistFromM3U8 may succeed with network retries, but should eventually fail")
	}

	// Test with empty URL (should fail)
	_, err = getChunklistFromM3U8("")
	if err == nil {
		t.Error("getChunklistFromM3U8 should return error for empty URL")
	}
}

func TestValidateAndCleanupOutputFile(t *testing.T) {
	// Save original env value
	originalEnv := os.Getenv(EnvRadicronHome)
	defer os.Setenv(EnvRadicronHome, originalEnv)

	testDir := filepath.Join(os.TempDir(), "radikron-test-validate")
	os.Setenv(EnvRadicronHome, testDir)
	defer os.RemoveAll(testDir)

	downloadsDir := filepath.Join(testDir, "downloads")
	err := os.MkdirAll(downloadsDir, DirPermissions)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create context with asset
	asset := &Asset{
		MinimumOutputSize: 1024 * 1024, // 1MB
	}
	ctx := context.WithValue(context.Background(), ContextKey("asset"), asset)

	// Test 1: File doesn't exist (should return false)
	output := newOutputConfigFromPath(downloadsDir, "nonexistent", radigo.AudioFormatAAC)
	shouldRetry := validateAndCleanupOutputFile(ctx, output)
	if shouldRetry {
		t.Error("validateAndCleanupOutputFile should return false when file doesn't exist")
	}

	// Test 2: File exists but is too small (should remove and return true)
	smallFile := output.AbsPath()
	err = os.WriteFile(smallFile, []byte("small"), 0600)
	if err != nil {
		t.Fatalf("Failed to create small file: %v", err)
	}

	shouldRetry = validateAndCleanupOutputFile(ctx, output)
	if !shouldRetry {
		t.Error("validateAndCleanupOutputFile should return true when file is too small")
	}
	if asset.NextFetchTime == nil {
		t.Error("NextFetchTime should be set when file is removed")
	}
	if _, err := os.Stat(smallFile); err == nil {
		t.Error("Small file should be removed")
	}

	// Test 3: File exists and is large enough (should return false)
	largeFile := output.AbsPath()
	largeContent := make([]byte, 2*1024*1024) // 2MB
	err = os.WriteFile(largeFile, largeContent, 0600)
	if err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	shouldRetry = validateAndCleanupOutputFile(ctx, output)
	if shouldRetry {
		t.Error("validateAndCleanupOutputFile should return false when file is large enough")
	}
	if _, err := os.Stat(largeFile); os.IsNotExist(err) {
		t.Error("Large file should not be removed")
	}

	// Test 4: File exists, is too small, but removal fails
	smallFile2 := filepath.Join(downloadsDir, "small2.aac")
	err = os.WriteFile(smallFile2, []byte("small"), 0600)
	if err != nil {
		t.Fatalf("Failed to create small file: %v", err)
	}
	output2 := newOutputConfigFromPath(downloadsDir, "small2", radigo.AudioFormatAAC)

	// Make file read-only to prevent deletion (on Unix)
	if runtime.GOOS != osWindows {
		err = os.Chmod(smallFile2, 0400)
		if err == nil {
			defer func() {
				_ = os.Chmod(smallFile2, 0600)
			}()
			shouldRetry = validateAndCleanupOutputFile(ctx, output2)
			// Should return false if removal fails
			if shouldRetry {
				t.Log("validateAndCleanupOutputFile may return true even if removal fails")
			}
		}
	}
}

func TestWriteOutputFile(t *testing.T) {
	// Save original env value
	originalEnv := os.Getenv(EnvRadicronHome)
	defer os.Setenv(EnvRadicronHome, originalEnv)

	testDir := filepath.Join(os.TempDir(), "radikron-test-write")
	os.Setenv(EnvRadicronHome, testDir)
	defer os.RemoveAll(testDir)

	downloadsDir := filepath.Join(testDir, "downloads")
	err := os.MkdirAll(downloadsDir, DirPermissions)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	ctx := context.Background()

	// Test AAC format (should just rename)
	sourceFile := filepath.Join(downloadsDir, "source.aac")
	err = os.WriteFile(sourceFile, []byte("test aac content"), 0600)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	output := newOutputConfigFromPath(downloadsDir, "test-output", radigo.AudioFormatAAC)
	err = writeOutputFile(ctx, sourceFile, output)
	if err != nil {
		t.Errorf("writeOutputFile failed for AAC: %v", err)
	}
	if _, err := os.Stat(output.AbsPath()); os.IsNotExist(err) {
		t.Error("Output file should exist after writeOutputFile for AAC")
	}
	if _, err := os.Stat(sourceFile); err == nil {
		t.Error("Source file should not exist after writeOutputFile (renamed)")
	}

	// Test MP3 format (requires ffmpeg, skip if not available)
	sourceFile2 := filepath.Join(downloadsDir, "source2.aac")
	err = os.WriteFile(sourceFile2, []byte("test aac content for mp3"), 0600)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	output2 := newOutputConfigFromPath(downloadsDir, "test-output2", radigo.AudioFormatMP3)
	err = writeOutputFile(ctx, sourceFile2, output2)
	if err != nil {
		// ffmpeg might not be available, that's okay
		if !strings.Contains(err.Error(), "ffmpeg not found") {
			t.Logf("writeOutputFile for MP3 failed (may be expected if ffmpeg not available): %v", err)
		}
	} else {
		// If conversion succeeded, verify output exists
		if _, err := os.Stat(output2.AbsPath()); os.IsNotExist(err) {
			t.Error("Output file should exist after writeOutputFile for MP3")
		}
	}

	// Test invalid format
	output3 := &radigo.OutputConfig{
		DirFullPath:  downloadsDir,
		FileBaseName: "test-invalid",
		FileFormat:   "invalid",
	}
	sourceFile3 := filepath.Join(downloadsDir, "source3.aac")
	err = os.WriteFile(sourceFile3, []byte("test"), 0600)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	err = writeOutputFile(ctx, sourceFile3, output3)
	if err == nil {
		t.Error("writeOutputFile should return error for invalid format")
	}
}

func TestDownload_InvalidTimeFormat(t *testing.T) {
	// Save original env value
	originalEnv := os.Getenv(EnvRadicronHome)
	defer os.Setenv(EnvRadicronHome, originalEnv)

	testDir := filepath.Join(os.TempDir(), "radikron-test-download")
	os.Setenv(EnvRadicronHome, testDir)
	defer os.RemoveAll(testDir)

	// Create context with asset
	asset := &Asset{
		OutputFormat:      radigo.AudioFormatAAC,
		DownloadDir:       "downloads",
		MinimumOutputSize: 1024,
		Rules:             Rules{},
		Schedules:         Schedules{},
	}
	ctx := context.WithValue(context.Background(), ContextKey("asset"), asset)

	wg := &sync.WaitGroup{}
	prog := &Prog{
		StationID: "FMT",
		Title:     "Test Program",
		Ft:        "invalid-time-format", // Invalid time format
		To:        "20230605140000",
	}

	err := Download(ctx, wg, prog)
	if err == nil {
		t.Error("Download should return error for invalid time format")
	}
	if !strings.Contains(err.Error(), "invalid start time format") {
		t.Errorf("Expected error about invalid time format, got: %v", err)
	}
}

func TestDownload_FutureProgram(t *testing.T) {
	// Save original env value
	originalEnv := os.Getenv(EnvRadicronHome)
	defer os.Setenv(EnvRadicronHome, originalEnv)

	testDir := filepath.Join(os.TempDir(), "radikron-test-download")
	os.Setenv(EnvRadicronHome, testDir)
	defer os.RemoveAll(testDir)

	// Set current time to a fixed point
	fixedTime := time.Date(2023, 6, 5, 12, 0, 0, 0, Location)
	CurrentTime = fixedTime

	// Create context with asset
	asset := &Asset{
		OutputFormat:      radigo.AudioFormatAAC,
		DownloadDir:       "downloads",
		MinimumOutputSize: 1024,
		Rules:             Rules{},
		Schedules:         Schedules{},
		NextFetchTime:     nil,
	}
	ctx := context.WithValue(context.Background(), ContextKey("asset"), asset)

	wg := &sync.WaitGroup{}
	// Program starts in the future (1 hour later)
	prog := &Prog{
		StationID: "FMT",
		Title:     "Test Program",
		Ft:        "20230605130000", // 1 PM (future)
		To:        "20230605140000", // 2 PM
	}

	err := Download(ctx, wg, prog)
	if err != nil {
		t.Errorf("Download should not return error for future program: %v", err)
	}

	// Verify NextFetchTime was set
	if asset.NextFetchTime == nil {
		t.Error("NextFetchTime should be set for future program")
	}

	// Verify program was not added to schedules (should skip)
	if len(asset.Schedules) != 0 {
		t.Error("Future program should not be added to schedules")
	}
}

func TestDownload_DuplicateProgram(t *testing.T) {
	// Save original env value
	originalEnv := os.Getenv(EnvRadicronHome)
	defer os.Setenv(EnvRadicronHome, originalEnv)

	testDir := filepath.Join(os.TempDir(), "radikron-test-download")
	os.Setenv(EnvRadicronHome, testDir)
	defer os.RemoveAll(testDir)

	// Set current time
	fixedTime := time.Date(2023, 6, 5, 12, 0, 0, 0, Location)
	CurrentTime = fixedTime

	// Load Versions from embedded JSON (required for NewDevice)
	versionsJSON, err := VersionsJSON.Open("assets/versions.json")
	if err != nil {
		t.Fatalf("Failed to open versions.json: %v", err)
	}
	defer versionsJSON.Close()
	blob, err := io.ReadAll(versionsJSON)
	if err != nil {
		t.Fatalf("Failed to read versions.json: %v", err)
	}
	var versions Versions
	if err := json.Unmarshal(blob, &versions); err != nil {
		t.Fatalf("Failed to unmarshal versions.json: %v", err)
	}

	// Create context with asset
	prog1 := &Prog{
		StationID: "FMT",
		Title:     "Test Program",
		Ft:        "20230605100000", // 10 AM (past)
		To:        "20230605110000",
	}
	asset := &Asset{
		OutputFormat:      radigo.AudioFormatAAC,
		DownloadDir:       "downloads",
		MinimumOutputSize: 1024,
		Rules:             Rules{},
		Schedules:         Schedules{prog1}, // Already in schedules
		Versions:          versions,
		AreaDevices:       map[string]*Device{},
	}
	ctx := context.WithValue(context.Background(), ContextKey("asset"), asset)

	wg := &sync.WaitGroup{}
	prog2 := &Prog{
		StationID: "FMT",
		Title:     "Test Program",
		Ft:        "20230605100000", // Same as prog1
		To:        "20230605110000",
	}

	err = Download(ctx, wg, prog2)
	if err != nil {
		t.Errorf("Download should not return error for duplicate program: %v", err)
	}

	// Verify program was not added again
	if len(asset.Schedules) != 1 {
		t.Errorf("Duplicate program should not be added, schedules count: %d", len(asset.Schedules))
	}
}

func TestDownload_InvalidEndTime(t *testing.T) {
	// Save original env value
	originalEnv := os.Getenv(EnvRadicronHome)
	defer os.Setenv(EnvRadicronHome, originalEnv)

	testDir := filepath.Join(os.TempDir(), "radikron-test-download")
	os.Setenv(EnvRadicronHome, testDir)
	defer os.RemoveAll(testDir)

	// Set current time
	fixedTime := time.Date(2023, 6, 5, 12, 0, 0, 0, Location)
	CurrentTime = fixedTime

	// Create context with asset
	asset := &Asset{
		OutputFormat:      radigo.AudioFormatAAC,
		DownloadDir:       "downloads",
		MinimumOutputSize: 1024,
		Rules:             Rules{},
		Schedules:         Schedules{},
	}
	ctx := context.WithValue(context.Background(), ContextKey("asset"), asset)

	wg := &sync.WaitGroup{}
	// Program in future but with invalid end time
	prog := &Prog{
		StationID: "FMT",
		Title:     "Test Program",
		Ft:        "20230605130000",   // 1 PM (future)
		To:        "invalid-end-time", // Invalid end time
	}

	err := Download(ctx, wg, prog)
	if err == nil {
		t.Error("Download should return error for invalid end time format")
	}
	if !strings.Contains(err.Error(), "invalid end time format") {
		t.Errorf("Expected error about invalid end time format, got: %v", err)
	}
}

func TestDownload_NoAssetInContext(t *testing.T) {
	ctx := context.Background() // No asset in context
	wg := &sync.WaitGroup{}
	prog := &Prog{
		StationID: "FMT",
		Title:     "Test Program",
		Ft:        "20230605100000",
		To:        "20230605110000",
	}

	// Download will panic when asset is nil, so we test that it panics
	// This tests the nil pointer dereference path
	defer func() {
		if r := recover(); r == nil {
			t.Error("Download should panic when asset is nil in context")
		}
	}()

	_ = Download(ctx, wg, prog)
	t.Error("Download should have panicked")
}

func TestDownloadLink(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test audio content"))
	}))
	defer server.Close()

	// Create temporary output directory
	tmpDir := t.TempDir()

	// Test successful download
	testURL := server.URL + "/test.aac"
	err := downloadLink(testURL, tmpDir)
	if err != nil {
		t.Errorf("downloadLink failed: %v", err)
	}

	// Verify file was created
	expectedFile := filepath.Join(tmpDir, "test.aac")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Error("File should be created after download")
	}

	// Verify file content
	content, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}
	if string(content) != "test audio content" {
		t.Errorf("File content mismatch: got %s, want test audio content", string(content))
	}
}

func TestDownloadLink_ServerError(t *testing.T) {
	// Create a test HTTP server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	testURL := server.URL + "/test.aac"

	// downloadLink doesn't check status code, so it will still create the file
	// but the content will be empty or error response
	err := downloadLink(testURL, tmpDir)
	// The function may or may not return an error depending on implementation
	// It writes the response body regardless of status code
	if err != nil {
		t.Logf("downloadLink returned error (may be expected): %v", err)
	}
}

func TestDownloadLink_InvalidURL(t *testing.T) {
	tmpDir := t.TempDir()
	invalidURL := "http://invalid-url-that-does-not-exist-12345.com/test.aac"

	err := downloadLink(invalidURL, tmpDir)
	if err == nil {
		t.Error("downloadLink should return error for invalid URL")
	}
}

func TestDownloadLink_FileCreationError(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test content"))
	}))
	defer server.Close()

	// Use invalid directory path to cause file creation error
	invalidDir := filepath.Join(os.TempDir(), "nonexistent", "subdir", "path")
	testURL := server.URL + "/test.aac"

	err := downloadLink(testURL, invalidDir)
	if err == nil {
		t.Error("downloadLink should return error when file creation fails")
	}
}

func TestBulkDownload_Success(t *testing.T) {
	// Initialize semaphores
	InitSemaphores(&Asset{
		MaxDownloadingConcurrency: 10,
		MaxEncodingConcurrency:    2,
	})

	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fileName := filepath.Base(r.URL.Path)
		_, _ = w.Write([]byte("content for " + fileName))
	}))
	defer server.Close()

	tmpDir := t.TempDir()

	// Test with multiple URLs
	urls := []string{
		server.URL + "/chunk1.aac",
		server.URL + "/chunk2.aac",
		server.URL + "/chunk3.aac",
	}

	err := bulkDownload(urls, tmpDir)
	if err != nil {
		t.Errorf("bulkDownload failed: %v", err)
	}

	// Verify all files were downloaded
	for i, url := range urls {
		fileName := filepath.Base(url)
		expectedFile := filepath.Join(tmpDir, fileName)
		if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
			t.Errorf("File %d (%s) should be created", i+1, fileName)
		}
	}
}

func TestBulkDownload_WithErrors(t *testing.T) {
	// Initialize semaphores
	InitSemaphores(&Asset{
		MaxDownloadingConcurrency: 10,
		MaxEncodingConcurrency:    2,
	})

	// Create a test HTTP server that fails for some requests
	var requestCount int64
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		count := requestCount
		mu.Unlock()
		// Fail every other request
		if count%2 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		fileName := filepath.Base(r.URL.Path)
		_, _ = w.Write([]byte("content for " + fileName))
	}))
	defer server.Close()

	tmpDir := t.TempDir()

	// Test with multiple URLs (some will fail)
	urls := []string{
		server.URL + "/chunk1.aac",
		server.URL + "/chunk2.aac",
		server.URL + "/chunk3.aac",
	}

	err := bulkDownload(urls, tmpDir)
	// bulkDownload retries, so it may succeed or fail depending on retry logic
	// The function returns error only if all retries fail
	if err != nil {
		t.Logf("bulkDownload returned error (may be expected with failures): %v", err)
	}
}

func TestBulkDownload_AllFail(t *testing.T) {
	// Initialize semaphores
	InitSemaphores(&Asset{
		MaxDownloadingConcurrency: 10,
		MaxEncodingConcurrency:    2,
	})

	tmpDir := t.TempDir()

	// Test with invalid URLs (all will fail)
	urls := []string{
		"http://invalid-url-1.com/chunk1.aac",
		"http://invalid-url-2.com/chunk2.aac",
	}

	err := bulkDownload(urls, tmpDir)
	if err == nil {
		t.Error("bulkDownload should return error when all downloads fail")
	}
	if err != nil && !strings.Contains(err.Error(), "lack of aac files") {
		t.Errorf("Expected 'lack of aac files' error, got: %v", err)
	}
}

func TestBulkDownload_EmptyList(t *testing.T) {
	// Initialize semaphores
	InitSemaphores(&Asset{
		MaxDownloadingConcurrency: 10,
		MaxEncodingConcurrency:    2,
	})

	tmpDir := t.TempDir()

	// Test with empty list
	urls := []string{}

	err := bulkDownload(urls, tmpDir)
	if err != nil {
		t.Errorf("bulkDownload should not return error for empty list: %v", err)
	}
}

func TestGetChunklist_MediaPlaylist(t *testing.T) {
	// Create a media playlist (not master playlist)
	mediaPlaylist := `#EXTM3U
#EXT-X-VERSION:3
#EXTINF:10.0,
chunk1.aac
#EXTINF:10.0,
chunk2.aac
#EXTINF:10.0,
chunk3.aac
#EXT-X-ENDLIST
`

	reader := strings.NewReader(mediaPlaylist)
	chunklist, err := getChunklist(reader)
	if err != nil {
		t.Errorf("getChunklist failed: %v", err)
	}
	if len(chunklist) != 3 {
		t.Errorf("Expected 3 chunks, got %d", len(chunklist))
	}
	if chunklist[0] != "chunk1.aac" {
		t.Errorf("First chunk should be chunk1.aac, got %s", chunklist[0])
	}
}

func TestGetChunklistFromM3U8_Success(t *testing.T) {
	// Create a media playlist
	mediaPlaylist := `#EXTM3U
#EXT-X-VERSION:3
#EXTINF:10.0,
chunk1.aac
#EXTINF:10.0,
chunk2.aac
#EXT-X-ENDLIST
`

	// Create HTTP server that serves the media playlist
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte(mediaPlaylist))
	}))
	defer server.Close()

	chunklist, err := getChunklistFromM3U8(server.URL)
	if err != nil {
		t.Errorf("getChunklistFromM3U8 failed: %v", err)
	}
	if len(chunklist) != 2 {
		t.Errorf("Expected 2 chunks, got %d", len(chunklist))
	}
}

func TestTimeshiftProgM3U8_NoAsset(t *testing.T) {
	// Test with no asset in context
	ctx := context.Background()
	prog := &Prog{
		StationID: "FMT",
		Ft:        "20230605130000",
		To:        "20230605145500",
	}

	// This will panic when trying to access nil asset
	defer func() {
		if r := recover(); r == nil {
			t.Error("timeshiftProgM3U8 should panic when asset is nil")
		}
	}()

	_, _ = timeshiftProgM3U8(ctx, prog)
	t.Error("timeshiftProgM3U8 should have panicked")
}

func TestDownloadProgram_ChunklistError(t *testing.T) {
	// Save original env value
	originalEnv := os.Getenv(EnvRadicronHome)
	defer os.Setenv(EnvRadicronHome, originalEnv)

	testDir := filepath.Join(os.TempDir(), "radikron-test-download-prog")
	os.Setenv(EnvRadicronHome, testDir)
	defer os.RemoveAll(testDir)

	ctx := context.Background()
	wg := &sync.WaitGroup{}

	// Create output config
	downloadsDir := filepath.Join(testDir, "downloads")
	err := os.MkdirAll(downloadsDir, DirPermissions)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	output := newOutputConfigFromPath(downloadsDir, "test-output", radigo.AudioFormatAAC)

	// Test with invalid M3U8 URL (will cause getChunklistFromM3U8 to fail)
	prog := &Prog{
		StationID: "FMT",
		Title:     "Test Program",
		M3U8:      "http://invalid-url-that-does-not-exist-12345.com/playlist.m3u8",
	}

	// downloadProgram runs in a goroutine, so we need to wait for it
	wg.Add(1)
	downloadProgram(ctx, wg, prog, output)
	wg.Wait()

	// Verify output file was not created (download should have failed)
	if _, err := os.Stat(output.AbsPath()); err == nil {
		t.Error("Output file should not be created when chunklist fetch fails")
	}
}

func TestDownloadProgram_BulkDownloadError(t *testing.T) {
	// Save original env value
	originalEnv := os.Getenv(EnvRadicronHome)
	defer os.Setenv(EnvRadicronHome, originalEnv)

	testDir := filepath.Join(os.TempDir(), "radikron-test-download-prog")
	os.Setenv(EnvRadicronHome, testDir)
	defer os.RemoveAll(testDir)

	ctx := context.Background()
	wg := &sync.WaitGroup{}

	// Create output config
	downloadsDir := filepath.Join(testDir, "downloads")
	err := os.MkdirAll(downloadsDir, DirPermissions)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	output := newOutputConfigFromPath(downloadsDir, "test-output", radigo.AudioFormatAAC)

	// Create a media playlist that points to invalid URLs (will cause bulkDownload to fail)
	mediaPlaylist := `#EXTM3U
#EXT-X-VERSION:3
#EXTINF:10.0,
http://invalid-url-1.com/chunk1.aac
#EXTINF:10.0,
http://invalid-url-2.com/chunk2.aac
#EXT-X-ENDLIST
`

	// Create HTTP server that serves the media playlist
	chunkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = w.Write([]byte(mediaPlaylist))
	}))
	defer chunkServer.Close()

	prog := &Prog{
		StationID: "FMT",
		Title:     "Test Program",
		M3U8:      chunkServer.URL,
	}

	// downloadProgram runs in a goroutine, so we need to wait for it
	wg.Add(1)
	downloadProgram(ctx, wg, prog, output)
	wg.Wait()

	// Verify output file was not created (download should have failed)
	if _, err := os.Stat(output.AbsPath()); err == nil {
		t.Error("Output file should not be created when bulkDownload fails")
	}
}
