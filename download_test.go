package radikron

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
		t.Errorf("newOutputConfig failed: %v", err)
	}
	if output == nil {
		t.Error("newOutputConfig returned nil")
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
		t.Errorf("newOutputConfig with folder failed: %v", err)
	}
	if output == nil {
		t.Error("newOutputConfig with folder returned nil")
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
