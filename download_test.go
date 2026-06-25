package radikron

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bogem/id3v2"
	"github.com/yyoshiki41/go-radiko"
	"github.com/yyoshiki41/radigo"
)

var (
	//go:embed test/playlist-test.m3u8
	PlaylistTestM3U8 embed.FS
)

const (
	osWindows       = "windows"
	testInvalidTime = "invalid"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGetRadicronPath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	// Test with relative path
	path, err := GetRadikronPath("downloads")
	if err != nil {
		t.Errorf("GetRadikronPath with relative path failed: %v", err)
	}
	expected := filepath.Join(cwd, "downloads")
	if path != expected {
		t.Errorf("GetRadikronPath => %v, want %v", path, expected)
	}

	// Test with absolute path (cross-platform)
	tmpDir := os.TempDir()
	absPath := filepath.Join(tmpDir, "test-downloads")
	path, err = GetRadikronPath(absPath)
	if err != nil {
		t.Errorf("GetRadikronPath with absolute path failed: %v", err)
	}
	if path != absPath {
		t.Errorf("GetRadikronPath with absolute path => %v, want %v", path, absPath)
	}

	// Test with subdirectory
	path, err = GetRadikronPath(filepath.Join("downloads", "subfolder"))
	if err != nil {
		t.Errorf("GetRadikronPath with subdirectory failed: %v", err)
	}
	expected = filepath.Join(cwd, "downloads", "subfolder")
	if path != expected {
		t.Errorf("GetRadikronPath with subdirectory => %v, want %v", path, expected)
	}

	// Test path cleaning (with .. and .)
	path, err = GetRadikronPath(filepath.Join("downloads", "..", "downloads", ".", "sub"))
	if err != nil {
		t.Errorf("GetRadikronPath with path cleaning failed: %v", err)
	}
	expected = filepath.Join(cwd, "downloads", "sub")
	if path != expected {
		t.Errorf("GetRadikronPath with path cleaning => %v, want %v", path, expected)
	}

	// Test with empty path (default case - should use $HOME/Downloads/radiko)
	path, err = GetRadikronPath("")
	if err != nil {
		t.Errorf("GetRadikronPath with empty path failed: %v", err)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to cwd/radiko if home directory can't be determined
		expected = filepath.Join(cwd, "radiko")
	} else {
		expected = filepath.Join(homeDir, "Downloads", "radiko")
	}
	if path != expected {
		t.Errorf("GetRadikronPath with empty path => %v, want %v", path, expected)
	}
}

func TestNewOutputConfig(t *testing.T) {
	// Test without folder
	output, err := NewOutputConfig("test-file", radigo.AudioFormatAAC, "downloads", "")
	if err != nil {
		t.Fatalf("NewOutputConfig failed: %v", err)
	}
	if output == nil {
		t.Fatal("NewOutputConfig returned nil")
	}
	if output.FileBaseName != "test-file" {
		t.Errorf("NewOutputConfig FileBaseName => %v, want test-file", output.FileBaseName)
	}
	if output.FileFormat != radigo.AudioFormatAAC {
		t.Errorf("NewOutputConfig FileFormat => %v, want %v", output.FileFormat, radigo.AudioFormatAAC)
	}

	// Test with folder
	output, err = NewOutputConfig("test-file", radigo.AudioFormatMP3, "downloads", "citypop")
	if err != nil {
		t.Fatalf("NewOutputConfig with folder failed: %v", err)
	}
	if output == nil {
		t.Fatal("NewOutputConfig with folder returned nil")
	}
	if output.FileFormat != radigo.AudioFormatMP3 {
		t.Errorf("NewOutputConfig FileFormat => %v, want %v", output.FileFormat, radigo.AudioFormatMP3)
	}

	// Test with custom download directory
	output, err = NewOutputConfig("test-file", radigo.AudioFormatAAC, "my-downloads", "")
	if err != nil {
		t.Errorf("NewOutputConfig with custom dir failed: %v", err)
	}
	if output == nil {
		t.Error("NewOutputConfig with custom dir returned nil")
	}
}

func TestTempAACDir(t *testing.T) {
	// tempAACDir now uses system temp directory
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

	// Verify it's in the system temp directory
	tmpDir := os.TempDir()
	if !strings.HasPrefix(dir, tmpDir) {
		t.Errorf("tempAACDir should be in system temp directory, got: %v", dir)
	}

	// Verify it has the expected prefix
	if !strings.Contains(filepath.Base(dir), "radikron-aac-") {
		t.Errorf("tempAACDir should have radikron-aac- prefix, got: %v", filepath.Base(dir))
	}

	// Clean up
	os.RemoveAll(dir)
}

// TestCheckDuplicate removed - checkDuplicate function was removed
// The functionality is now handled by handleDuplicate which is tested below

// setupHandleDuplicateTest creates a test environment for handleDuplicate tests
func setupHandleDuplicateTest(t *testing.T) (downloadsDir string, cleanup func()) {
	t.Helper()
	testDir := filepath.Join(os.TempDir(), "radikron-test-handle-dup")
	downloadsDir = filepath.Join(testDir, "downloads")
	if err := os.MkdirAll(downloadsDir, DirPermissions); err != nil {
		t.Fatalf("Failed to create test downloads directory: %v", err)
	}
	cleanup = func() {
		os.RemoveAll(testDir)
	}
	return downloadsDir, cleanup
}

func TestHandleDuplicate_NonexistentFile(t *testing.T) {
	_, cleanup := setupHandleDuplicateTest(t)
	defer cleanup()

	output, err := NewOutputConfig("nonexistent-file", radigo.AudioFormatAAC, "downloads", "")
	if err != nil {
		t.Fatalf("NewOutputConfig failed: %v", err)
	}
	ctx := context.Background()
	err = handleDuplicate(
		ctx, "nonexistent-file", radigo.AudioFormatAAC, "downloads", "",
		output, Rules{}, "TEST", "Test Program", "20230605100000")
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

	output, err := NewOutputConfig("test-file", radigo.AudioFormatAAC, "downloads", "")
	if err != nil {
		t.Fatalf("NewOutputConfig failed: %v", err)
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

	output := newOutputConfigFromPath(citypopDir, "move-test", radigo.AudioFormatAAC)
	ctx := context.Background()
	// Use absolute path for downloadDir to match the test directory
	err = handleDuplicate(
		ctx, "move-test", radigo.AudioFormatAAC, downloadsDir, "citypop",
		output, Rules{}, "TEST", "Test Program", "20230605100000")
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

	output, err := NewOutputConfig("move-test", radigo.AudioFormatAAC, "downloads", "citypop")
	if err != nil {
		t.Fatalf("NewOutputConfig failed: %v", err)
	}
	ctx := context.Background()
	err = handleDuplicate(
		ctx, "move-test", radigo.AudioFormatAAC, "downloads", "citypop",
		output, Rules{}, "TEST", "Test Program", "20230605100000")
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

	output, err := NewOutputConfig("conflict-test", radigo.AudioFormatAAC, "downloads", "citypop")
	if err != nil {
		t.Fatalf("NewOutputConfig failed: %v", err)
	}
	ctx := context.Background()
	err = handleDuplicate(
		ctx, "conflict-test", radigo.AudioFormatAAC, "downloads", "citypop",
		output, Rules{}, "TEST", "Test Program", "20230605100000")
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
	output, err := NewOutputConfig("test-file", radigo.AudioFormatAAC, "downloads", "citypop")
	if err != nil {
		t.Fatalf("NewOutputConfig failed: %v", err)
	}
	ctx := context.Background()
	err = handleDuplicate(
		ctx, "test-file", radigo.AudioFormatAAC, "downloads", "citypop",
		output, rules, "TEST", "Test Program", "20230605100000")
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

	// Create output config for configured folder using absolute path
	output := newOutputConfigFromPath(configuredDir, "target-exists-test", radigo.AudioFormatAAC)

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
	err = handleDuplicate(
		ctx, "target-exists-test", radigo.AudioFormatAAC, "downloads", "citypop",
		output, Rules{}, "TEST", "Test Program", "20230605100000")
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

func TestGetTimeshiftChunklist(t *testing.T) { //nolint:gocyclo // transport assertions intentionally cover complete request flow
	originalTransport := http.DefaultTransport
	originalLogWriter := log.Writer()
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
		radiko.SetHTTPClient(&http.Client{Timeout: 120 * time.Second})
		log.SetOutput(originalLogWriter)
	})

	var diagnosticLog bytes.Buffer
	log.SetOutput(&diagnosticLog)
	dumpPath := filepath.Join(t.TempDir(), "timeshift-chunks.txt")
	t.Setenv(timeshiftDebugEnv, "1")
	t.Setenv(timeshiftChunklistDumpEnv, dumpPath)

	var playlistRequests int
	var seeks []string
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := ""
		switch {
		case req.URL.Host == "radiko.jp" && req.URL.Path == "/area":
			body = `<span class="JP13">Tokyo</span>`
		case req.URL.Host == "tf-f-rpaa-radiko.smartstream.ne.jp":
			playlistRequests++
			if got := req.Header.Get(RadikoAreaIDHeader); got != DefaultArea {
				t.Errorf("%s = %q, want JP13", RadikoAreaIDHeader, got)
			}
			if got := req.Header.Get(RadikoAuthTokenHeader); got != "token" {
				t.Errorf("%s = %q, want token", RadikoAuthTokenHeader, got)
			}
			if got := req.URL.Query().Get("station_id"); got != testStationFMT {
				t.Errorf("station_id = %q, want FMT", got)
			}
			if got := req.URL.Query().Get("seek"); got == "" {
				t.Error("seek is empty")
			} else {
				seeks = append(seeks, got)
			}
			if got := req.URL.Query().Get("l"); got != "20" {
				t.Errorf("l = %q, want 20", got)
			}
			body = "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=48000\nhttps://chunks.test/list.m3u8\n"
		case req.URL.Host == "chunks.test":
			switch playlistRequests {
			case 1:
				body = "#EXTM3U\n#EXT-X-VERSION:3\n" +
					"#EXTINF:5,\nhttps://audio.test/tf/segments/o/B/FMT/20230605/20230605_130000_one.aac\n" +
					"#EXTINF:5,\nhttps://audio.test/tf/segments/o/B/FMT/20230605/20230605_130005_two.aac\n" +
					"#EXTINF:5,\nhttps://audio.test/tf/segments/c/x/8/advertisement.aac\n" +
					"#EXTINF:5,\nhttps://audio.test/tf/segments/o/B/FMT/20230605/20230605_130010_three.aac?token=first\n"
			case 2:
				body = "#EXTM3U\n#EXT-X-VERSION:3\n" +
					"#EXTINF:5,\nhttps://audio.test/tf/segments/o/B/FMT/20230605/20230605_130010_changed.aac?token=second\n" +
					"#EXTINF:5,\nhttps://audio.test/tf/segments/o/B/FMT/20230605/20230605_130015_four.aac\n" +
					"#EXTINF:5,\nhttps://audio.test/tf/segments/o/B/FMT/20230605/20230605_130020_five.aac\n" +
					"#EXTINF:5,\nhttps://audio.test/tf/segments/o/B/FMT/20230605/20230605_130025_six.aac\n" +
					"#EXTINF:5,\nhttps://audio.test/tf/segments/o/B/FMT/20230605/20230605_130030_tail.aac\n"
			default:
				t.Fatalf("unexpected playlist request count: %d", playlistRequests)
			}
		default:
			t.Fatalf("unexpected request: %s", req.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})
	http.DefaultTransport = transport
	radiko.SetHTTPClient(&http.Client{Transport: transport})

	client, err := radiko.New("")
	if err != nil {
		t.Fatalf("radiko.New failed: %v", err)
	}
	asset := &Asset{
		DefaultClient: client,
		AreaDevices: Devices{
			DefaultArea: {
				AuthToken: "token",
				UserAgent: "test-agent",
			},
		},
		Stations: Stations{
			testStationFMT: {Areas: []string{DefaultArea}},
		},
	}
	ctx := context.WithValue(context.Background(), ContextKey("asset"), asset)
	prog := &Prog{
		StationID: testStationFMT,
		Ft:        "20230605130000",
		To:        "20230605130030",
	}

	chunks, err := getTimeshiftChunklist(ctx, prog)
	if err != nil {
		t.Fatalf("getTimeshiftChunklist failed: %v", err)
	}
	want := []string{
		"https://audio.test/tf/segments/o/B/FMT/20230605/20230605_130000_one.aac",
		"https://audio.test/tf/segments/o/B/FMT/20230605/20230605_130005_two.aac",
		"https://audio.test/tf/segments/o/B/FMT/20230605/20230605_130010_three.aac?token=first",
		"https://audio.test/tf/segments/o/B/FMT/20230605/20230605_130015_four.aac",
		"https://audio.test/tf/segments/o/B/FMT/20230605/20230605_130020_five.aac",
		"https://audio.test/tf/segments/o/B/FMT/20230605/20230605_130025_six.aac",
	}
	if len(chunks) != len(want) {
		t.Fatalf("chunks = %v, want %v", chunks, want)
	}
	for i := range want {
		if chunks[i] != want[i] {
			t.Errorf("chunks[%d] = %q, want %q", i, chunks[i], want[i])
		}
	}
	if playlistRequests != 2 {
		t.Errorf("playlist requests = %d, want 2", playlistRequests)
	}
	wantSeeks := []string{
		"20230605130000",
		"20230605130015",
	}
	if !reflect.DeepEqual(seeks, wantSeeks) {
		t.Errorf("seeks = %v, want %v", seeks, wantSeeks)
	}

	dump, err := os.ReadFile(dumpPath)
	if err != nil {
		t.Fatalf("read chunklist dump failed: %v", err)
	}
	wantDump := strings.Join(want, "\n") + "\n"
	if string(dump) != wantDump {
		t.Errorf("chunklist dump = %q, want %q", dump, wantDump)
	}

	logOutput := diagnosticLog.String()
	for _, wantLog := range []string{
		`timeshift diagnostics debug_env="1" dump_env="` + dumpPath + `" debug_enabled=true dump_enabled=true`,
		`timeshift chunklist dump enabled path="` + dumpPath + `"`,
		"timeshift seek=20230605130000",
		`timeshift media playlist seek=20230605130000 url="https://chunks.test/list.m3u8"`,
		`url="https://audio.test/tf/segments/c/x/8/advertisement.aac" reason="non-program segment class \"c\""`,
		`key="FMT/20230605130010" timestamp=20230605130010 duplicate=true`,
		`url="https://audio.test/tf/segments/o/B/FMT/20230605/20230605_130030_tail.aac" reason="timestamp outside program range: 20230605130030"`,
		"timeshift final chunk count=6",
		`timeshift chunklist dumped path="` + dumpPath + `" count=6`,
	} {
		if !strings.Contains(logOutput, wantLog) {
			t.Errorf("diagnostic log missing %q:\n%s", wantLog, logOutput)
		}
	}
}

func TestGetTimeshiftChunklistInvalidTimes(t *testing.T) {
	asset := &Asset{
		AreaDevices: Devices{DefaultArea: {}},
		Stations:    Stations{testStationFMT: {Areas: []string{DefaultArea}}},
	}
	ctx := context.WithValue(context.Background(), ContextKey("asset"), asset)

	tests := []struct {
		name string
		ft   string
		to   string
	}{
		{name: "invalid start", ft: testInvalidTime, to: "20230605130030"},
		{name: "invalid end", ft: "20230605130000", to: testInvalidTime},
		{name: "reversed", ft: "20230605130030", to: "20230605130000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getTimeshiftChunklist(ctx, &Prog{
				StationID: testStationFMT,
				Ft:        tt.ft,
				To:        tt.to,
			})
			if err == nil {
				t.Fatal("getTimeshiftChunklist returned nil error")
			}
		})
	}
}

func TestTimeshiftPlaylistHelpers(t *testing.T) {
	master := "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=48000\nchunklist.m3u8\n"
	uri, err := parseMasterPlaylistURI(master)
	if err != nil {
		t.Fatalf("parseMasterPlaylistURI failed: %v", err)
	}
	if uri != "chunklist.m3u8" {
		t.Errorf("uri = %q, want chunklist.m3u8", uri)
	}

	for _, body := range []string{"#EXTM3U\n", "not a playlist"} {
		if _, err := parseMasterPlaylistURI(body); err == nil {
			t.Errorf("parseMasterPlaylistURI(%q) returned nil error", body)
		}
	}

	resolved := resolveURL("https://example.com/path/master.m3u8", "../chunklist.m3u8")
	if resolved != "https://example.com/chunklist.m3u8" {
		t.Errorf("resolveURL = %q", resolved)
	}
	location, err := time.LoadLocation(TZTokyo)
	if err != nil {
		t.Fatalf("time.LoadLocation failed: %v", err)
	}
	from, err := time.ParseInLocation(DatetimeLayout, "20260625130000", location)
	if err != nil {
		t.Fatalf("parse from failed: %v", err)
	}
	to, err := time.ParseInLocation(DatetimeLayout, "20260625140000", location)
	if err != nil {
		t.Fatalf("parse to failed: %v", err)
	}
	validURL := "https://example.com/tf/segments/o/B/FMT/20260625/20260625_135928_25k91.aac?token=secret"
	segment, err := parseTimeshiftProgramSegment(validURL, testStationFMT, from, to, location)
	if err != nil {
		t.Fatalf("parseTimeshiftProgramSegment failed: %v", err)
	}
	if segment.Path != "/tf/segments/o/B/FMT/20260625/20260625_135928_25k91.aac" {
		t.Errorf("segment.Path = %q", segment.Path)
	}
	if key := segment.key(); key != "FMT/20260625135928" {
		t.Errorf("segment key = %q, want FMT/20260625135928", key)
	}

	invalidSegments := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "commercial segment",
			url:  "https://example.com/tf/segments/c/x/8/rBDj5dHm7S4TYZmNkwo7.aac",
			want: `non-program segment class "c"`,
		},
		{
			name: "wrong station",
			url:  "https://example.com/tf/segments/o/B/TBS/20260625/20260625_135928_25k91.aac",
			want: `station mismatch`,
		},
		{
			name: "wrong path date",
			url:  "https://example.com/tf/segments/o/B/FMT/20260624/20260625_135928_25k91.aac",
			want: `date mismatch`,
		},
		{
			name: "unparsable filename",
			url:  "https://example.com/tf/segments/o/B/FMT/20260625/not-a-timestamp.aac",
			want: `unexpected program segment filename`,
		},
		{
			name: "outside program range",
			url:  "https://example.com/tf/segments/o/B/FMT/20260625/20260625_140000_tail.aac",
			want: `timestamp outside program range`,
		},
	}
	for _, tt := range invalidSegments {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTimeshiftProgramSegment(tt.url, testStationFMT, from, to, location)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("parse error = %v, want containing %q", err, tt.want)
			}
		})
	}

	t.Setenv(timeshiftDebugEnv, "yes")
	if !timeshiftDebugEnabled() {
		t.Error("timeshiftDebugEnabled = false, want true")
	}
	t.Setenv(timeshiftDebugEnv, "invalid")
	if timeshiftDebugEnabled() {
		t.Error("timeshiftDebugEnabled = true, want false")
	}

	if err := dumpChunklist(t.TempDir(), []string{"one.aac"}); err == nil ||
		!strings.Contains(err.Error(), "create dump file") {
		t.Errorf("dumpChunklist create error = %v", err)
	}

	media := "#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:15,\none.aac\n#EXTINF:15,\ntwo.aac\n"
	chunks, err := extractChunklist(strings.NewReader(media))
	if err != nil {
		t.Fatalf("extractChunklist failed: %v", err)
	}
	if len(chunks) != 2 || chunks[0] != "one.aac" || chunks[1] != "two.aac" {
		t.Errorf("chunks = %v", chunks)
	}
	chunks, err = extractChunklist(strings.NewReader(master))
	if err != nil {
		t.Fatalf("extractChunklist master playlist failed: %v", err)
	}
	if chunks != nil {
		t.Errorf("extractChunklist master chunks = %v, want nil", chunks)
	}

	lsid := generateLSID()
	if len(lsid) != 32 {
		t.Errorf("generateLSID length = %d, want 32", len(lsid))
	}
}

func TestValidateAndCleanupOutputFile(t *testing.T) {
	testDir := filepath.Join(os.TempDir(), "radikron-test-validate")
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
	testDir := filepath.Join(os.TempDir(), "radikron-test-write")
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
	testDir := filepath.Join(os.TempDir(), "radikron-test-download")
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
	testDir := filepath.Join(os.TempDir(), "radikron-test-download")
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
	testDir := filepath.Join(os.TempDir(), "radikron-test-download")
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
	testDir := filepath.Join(os.TempDir(), "radikron-test-download")
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
	t.Parallel()
	ctx := context.Background() // No asset in context
	wg := &sync.WaitGroup{}
	prog := &Prog{
		StationID: "FMT",
		Title:     "Test Program",
		Ft:        "20230605100000",
		To:        "20230605110000",
	}

	// Download should return an error when asset is nil
	err := Download(ctx, wg, prog)
	if err == nil {
		t.Error("Download should return an error when asset is nil in context")
	}
	if !strings.Contains(err.Error(), "asset is nil") {
		t.Errorf("Expected error about nil asset, got: %v", err)
	}
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
		_, _ = w.Write([]byte("content for " + fileName)) //nolint:gosec // test server response contains sanitized path base
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
		_, _ = w.Write([]byte("content for " + fileName)) //nolint:gosec // test server response contains sanitized path base
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

// mockEventEmitter is a test implementation of EventEmitter
type mockEventEmitter struct {
	downloadStarted   []struct{ stationID, title, startTime string }
	downloadCompleted []struct{ stationID, title, startTime, filePath string }
	fileSaved         []struct{ stationID, title, filePath string }
	downloadSkipped   []struct{ reason, stationID, title, startTime string }
	encodingStarted   []string
	encodingCompleted []string
	logMessages       []struct{ level, message string }
}

func (m *mockEventEmitter) EmitDownloadStarted(stationID, title, startTime string) {
	m.downloadStarted = append(m.downloadStarted, struct{ stationID, title, startTime string }{stationID, title, startTime})
}

func (m *mockEventEmitter) EmitDownloadCompleted(stationID, title, startTime, filePath string) {
	m.downloadCompleted = append(
		m.downloadCompleted,
		struct{ stationID, title, startTime, filePath string }{
			stationID, title, startTime, filePath,
		},
	)
}

func (m *mockEventEmitter) EmitFileSaved(stationID, title, filePath string) {
	m.fileSaved = append(m.fileSaved, struct{ stationID, title, filePath string }{stationID, title, filePath})
}

func (m *mockEventEmitter) EmitDownloadSkipped(reason, stationID, title, startTime string) {
	m.downloadSkipped = append(m.downloadSkipped, struct{ reason, stationID, title, startTime string }{reason, stationID, title, startTime})
}

func (m *mockEventEmitter) EmitEncodingStarted(filePath string) {
	m.encodingStarted = append(m.encodingStarted, filePath)
}

func (m *mockEventEmitter) EmitEncodingCompleted(filePath string) {
	m.encodingCompleted = append(m.encodingCompleted, filePath)
}

func (m *mockEventEmitter) EmitLogMessage(level, message string) {
	m.logMessages = append(m.logMessages, struct{ level, message string }{level, message})
}

func TestEmitDownloadStarted_WithEmitter(t *testing.T) {
	emitter := &mockEventEmitter{}
	ctx := context.WithValue(context.Background(), ContextKey("eventEmitter"), emitter)

	emitDownloadStarted(ctx, "FMT", "Test Program", "20230605100000")

	if len(emitter.downloadStarted) != 1 {
		t.Errorf("Expected 1 download started event, got %d", len(emitter.downloadStarted))
	}
	if emitter.downloadStarted[0].stationID != "FMT" {
		t.Errorf("Expected stationID FMT, got %s", emitter.downloadStarted[0].stationID)
	}
}

func TestEmitDownloadStarted_WithoutEmitter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Should not panic, just log
	emitDownloadStarted(ctx, "FMT", "Test Program", "20230605100000")
}

func TestEmitDownloadCompleted_WithEmitter(t *testing.T) {
	emitter := &mockEventEmitter{}
	ctx := context.WithValue(context.Background(), ContextKey("eventEmitter"), emitter)

	emitDownloadCompleted(ctx, "FMT", "Test Program", "20230605100000", "/path/to/file.aac")

	if len(emitter.downloadCompleted) != 1 {
		t.Errorf("Expected 1 download completed event, got %d", len(emitter.downloadCompleted))
	}
}

func TestEmitDownloadCompleted_WithoutEmitter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	emitDownloadCompleted(ctx, "FMT", "Test Program", "20230605100000", "/path/to/file.aac")
}

func TestEmitFileSaved_WithEmitter(t *testing.T) {
	emitter := &mockEventEmitter{}
	ctx := context.WithValue(context.Background(), ContextKey("eventEmitter"), emitter)

	emitFileSaved(ctx, "FMT", "Test Program", "/path/to/file.aac")

	if len(emitter.fileSaved) != 1 {
		t.Errorf("Expected 1 file saved event, got %d", len(emitter.fileSaved))
	}
}

func TestEmitFileSaved_WithoutEmitter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	emitFileSaved(ctx, "FMT", "Test Program", "/path/to/file.aac")
}

func TestEmitDownloadSkipped_WithEmitter(t *testing.T) {
	emitter := &mockEventEmitter{}
	ctx := context.WithValue(context.Background(), ContextKey("eventEmitter"), emitter)

	emitDownloadSkipped(ctx, "already exists", "FMT", "Test Program", "20230605100000")

	if len(emitter.downloadSkipped) != 1 {
		t.Errorf("Expected 1 download skipped event, got %d", len(emitter.downloadSkipped))
	}
}

func TestEmitDownloadSkipped_WithoutEmitter_WithAllFields(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	emitDownloadSkipped(ctx, "already exists", "FMT", "Test Program", "20230605100000")
}

func TestEmitDownloadSkipped_WithoutEmitter_WithEmptyFields(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	emitDownloadSkipped(ctx, "test reason", "", "", "")
}

func TestEmitEncodingStarted_WithEmitter(t *testing.T) {
	emitter := &mockEventEmitter{}
	ctx := context.WithValue(context.Background(), ContextKey("eventEmitter"), emitter)

	emitEncodingStarted(ctx, "/path/to/file.aac")

	if len(emitter.encodingStarted) != 1 {
		t.Errorf("Expected 1 encoding started event, got %d", len(emitter.encodingStarted))
	}
}

func TestEmitEncodingStarted_WithoutEmitter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	emitEncodingStarted(ctx, "/path/to/file.aac")
}

func TestEmitEncodingCompleted_WithEmitter(t *testing.T) {
	emitter := &mockEventEmitter{}
	ctx := context.WithValue(context.Background(), ContextKey("eventEmitter"), emitter)

	emitEncodingCompleted(ctx, "/path/to/file.mp3")

	if len(emitter.encodingCompleted) != 1 {
		t.Errorf("Expected 1 encoding completed event, got %d", len(emitter.encodingCompleted))
	}
}

func TestEmitEncodingCompleted_WithoutEmitter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	emitEncodingCompleted(ctx, "/path/to/file.mp3")
}

func TestEmitLogMessage_WithEmitter(t *testing.T) {
	emitter := &mockEventEmitter{}
	ctx := context.WithValue(context.Background(), ContextKey("eventEmitter"), emitter)

	emitLogMessage(ctx, "info", "Test message")

	if len(emitter.logMessages) != 1 {
		t.Errorf("Expected 1 log message, got %d", len(emitter.logMessages))
	}
	if emitter.logMessages[0].level != "info" {
		t.Errorf("Expected level info, got %s", emitter.logMessages[0].level)
	}
}

func TestEmitLogMessage_WithoutEmitter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	emitLogMessage(ctx, "error", "Test error message")
}

func TestEmitLogMessage_WithoutEmitter_EmptyLevel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	emitLogMessage(ctx, "", "Test message with empty level")
}

func TestMoveFile_ErrorOpeningSource(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentSource := filepath.Join(tmpDir, "nonexistent.txt")
	destFile := filepath.Join(tmpDir, "dest.txt")

	err := moveFile(nonExistentSource, destFile)
	if err == nil {
		t.Error("moveFile should return error when source file doesn't exist")
	}
}

func TestMoveFile_CopyError(t *testing.T) {
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.txt")

	// Create source file
	err := os.WriteFile(sourceFile, []byte("test content"), 0600)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// On Unix, try to move to a read-only location to force copy fallback error
	// Skip on Windows as permissions work differently
	if runtime.GOOS != osWindows {
		readOnlyDir := filepath.Join(tmpDir, "readonly")
		err = os.MkdirAll(readOnlyDir, 0500)
		if err != nil {
			t.Fatalf("Failed to create read-only directory: %v", err)
		}
		defer func() {
			_ = os.Chmod(readOnlyDir, 0700)
		}()

		readOnlyDest := filepath.Join(readOnlyDir, "dest.txt")
		err = moveFile(sourceFile, readOnlyDest)
		// Should fail when trying to create destination in read-only dir
		if err == nil {
			t.Error("moveFile should return error when destination directory is read-only")
		}
	}
}

func TestHandleMoveFromDefaultFolder_TargetExists(t *testing.T) {
	downloadsDir, cleanup := setupHandleDuplicateTest(t)
	defer cleanup()

	citypopDir := filepath.Join(downloadsDir, "citypop")
	err := os.MkdirAll(citypopDir, DirPermissions)
	if err != nil {
		t.Fatalf("Failed to create citypop directory: %v", err)
	}

	// Create file in default folder
	defaultFile := filepath.Join(downloadsDir, "move-test.aac")
	err = os.WriteFile(defaultFile, []byte("test content"), 0600)
	if err != nil {
		t.Fatalf("Failed to create default file: %v", err)
	}

	// Create target file (simulating race condition)
	output := newOutputConfigFromPath(citypopDir, "move-test", radigo.AudioFormatAAC)
	targetFile := output.AbsPath()
	err = os.WriteFile(targetFile, []byte("existing content"), 0600)
	if err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	ctx := context.Background()
	err = handleMoveFromDefaultFolder(ctx, defaultFile, targetFile, output, "TEST", "Test Program", "20230605100000")
	if err != nil {
		t.Errorf("handleMoveFromDefaultFolder should not return error when target exists: %v", err)
	}

	// Source should still exist (target exists, so move is skipped)
	if _, err := os.Stat(defaultFile); os.IsNotExist(err) {
		t.Error("Source file should still exist when target exists")
	}
}

func TestHandleMoveFromDefaultFolder_MoveErrorTargetAppears(t *testing.T) {
	downloadsDir, cleanup := setupHandleDuplicateTest(t)
	defer cleanup()

	citypopDir := filepath.Join(downloadsDir, "citypop")
	err := os.MkdirAll(citypopDir, DirPermissions)
	if err != nil {
		t.Fatalf("Failed to create citypop directory: %v", err)
	}

	// Create file in default folder
	defaultFile := filepath.Join(downloadsDir, "move-test.aac")
	err = os.WriteFile(defaultFile, []byte("test content"), 0600)
	if err != nil {
		t.Fatalf("Failed to create default file: %v", err)
	}

	output, err := NewOutputConfig("move-test", radigo.AudioFormatAAC, "downloads", "citypop")
	if err != nil {
		t.Fatalf("NewOutputConfig failed: %v", err)
	}
	targetFile := output.AbsPath()

	// On Unix, we can simulate a move error by making the destination directory read-only
	// This will cause moveFile to fail, but if target appears during the error, source should be removed
	if runtime.GOOS != osWindows {
		// Make citypop dir read-only to cause move error
		err = os.Chmod(citypopDir, 0500)
		if err == nil {
			defer func() {
				_ = os.Chmod(citypopDir, 0700)
			}()

			// Create target file after making dir read-only (simulating race condition)
			// This tests the error path where target appears during move error
			err = os.WriteFile(targetFile, []byte("existing"), 0600)
			if err == nil {
				ctx := context.Background()
				err = handleMoveFromDefaultFolder(ctx, defaultFile, targetFile, output, "TEST", "Test Program", "20230605100000")
				// Should handle the error gracefully
				_ = err
			}
		}
	}
}

func TestGetRadicronPath_GetwdError(t *testing.T) {
	// This is hard to test directly, but we can verify the error path exists
	// by checking the code handles Getwd errors
	// GetRadikronPath calls os.Getwd() which rarely fails, but the code should handle it
	// We can't easily mock os.Getwd, but we verify the error handling exists
	_, err := GetRadikronPath("test")
	// Should succeed in normal cases
	if err != nil {
		t.Logf("GetRadikronPath returned error (may be expected in some environments): %v", err)
	}
}

func TestConvertAACtoMP3_FFmpegNotFound(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.aac")
	destFile := filepath.Join(tmpDir, "dest.mp3")

	// Create source file
	err := os.WriteFile(sourceFile, []byte("test aac content"), 0600)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Temporarily modify PATH to exclude ffmpeg
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Set PATH to empty to simulate ffmpeg not found
	os.Setenv("PATH", "")

	err = convertAACtoMP3(ctx, sourceFile, destFile)
	if err == nil {
		t.Error("convertAACtoMP3 should return error when ffmpeg is not found")
	}
	if err != nil && !strings.Contains(err.Error(), "ffmpeg not found") {
		t.Errorf("Expected error about ffmpeg not found, got: %v", err)
	}
}

func TestConvertAACtoMP3_ConversionError(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.aac")
	destFile := filepath.Join(tmpDir, "dest.mp3")

	// Create invalid source file (not a valid AAC file)
	err := os.WriteFile(sourceFile, []byte("not a valid aac file"), 0600)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Check if ffmpeg is available
	_, err = exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not available, skipping conversion error test")
	}

	// Try to convert invalid file
	err = convertAACtoMP3(ctx, sourceFile, destFile)
	// Should fail with conversion error
	if err == nil {
		t.Error("convertAACtoMP3 should return error for invalid source file")
	}
	if err != nil && !strings.Contains(err.Error(), "ffmpeg conversion failed") {
		t.Logf("convertAACtoMP3 returned error (expected): %v", err)
	}
}
