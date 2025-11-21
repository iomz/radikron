package radikron

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bogem/id3v2"
	"github.com/yyoshiki41/radigo"
)

var (
	//go:embed test/playlist-test.m3u8
	PlaylistTestM3U8 embed.FS
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
	err := os.MkdirAll(tmpDir, 0755)
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
}

func TestCheckDuplicate(t *testing.T) {
	// Save original env value
	originalEnv := os.Getenv(EnvRadicronHome)
	defer os.Setenv(EnvRadicronHome, originalEnv)

	// Set a test directory
	testDir := filepath.Join(os.TempDir(), "radikron-test-dup")
	os.Setenv(EnvRadicronHome, testDir)
	defer os.RemoveAll(testDir)

	// Create directory structure
	downloadsDir := filepath.Join(testDir, "downloads")
	err := os.MkdirAll(downloadsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test downloads directory: %v", err)
	}

	// Test with non-existent file (should return false)
	exists, path := checkDuplicate("nonexistent-file", radigo.AudioFormatAAC, "downloads", "")
	if exists {
		t.Errorf("checkDuplicate returned true for non-existent file: %v", path)
	}

	// Test with configured folder
	exists, path = checkDuplicate("nonexistent-file", radigo.AudioFormatAAC, "downloads", "citypop")
	if exists {
		t.Errorf("checkDuplicate returned true for non-existent file in folder: %v", path)
	}

	// Test with existing file
	testFile := filepath.Join(downloadsDir, "test-file.aac")
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	file.Close()

	exists, path = checkDuplicate("test-file", radigo.AudioFormatAAC, "downloads", "")
	if !exists {
		t.Error("checkDuplicate should return true for existing file")
	}
	if path == "" {
		t.Error("checkDuplicate should return path for existing file")
	}

	// Clean up
	os.Remove(testFile)
}

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
	err = handleDuplicate("nonexistent-file", radigo.AudioFormatAAC, "downloads", "", output)
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
	err = handleDuplicate("test-file", radigo.AudioFormatAAC, "downloads", "", output)
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
	err = handleDuplicate("move-test", radigo.AudioFormatAAC, "downloads", "citypop", output)
	if err != nil {
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
	err = handleDuplicate("move-test", radigo.AudioFormatAAC, "downloads", "citypop", output)
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
	err = handleDuplicate("conflict-test", radigo.AudioFormatAAC, "downloads", "citypop", output)
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
