package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/iomz/radikron"
	"github.com/yyoshiki41/go-radiko"
	"github.com/yyoshiki41/radigo"
)

const (
	testStationID         = "FMT"
	testConfigFile        = "test/config-test.yml"
	nonexistentConfigFile = "nonexistent-config.yml"
)

func TestConfig(t *testing.T) {
	var err error
	radikron.Location, err = time.LoadLocation(radikron.TZTokyo)
	if err != nil {
		t.Error(err)
	}
	client, err := radiko.New("")
	if err != nil {
		log.Fatal(err)
	}
	asset, err := radikron.NewAsset(client)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), contextKey, asset)
	cfg, err := reloadConfig(ctx, testConfigFile, time.Now, defaultTimeSetter)
	if err != nil {
		t.Error(err)
	}

	if asset.OutputFormat != radigo.AudioFormatAAC {
		t.Errorf("%v => want %v", asset.OutputFormat, radigo.AudioFormatAAC)
	}

	if len(cfg.Rules) != 4 {
		t.Error("error parsing the rules")
	}

	got := len(asset.AvailableStations)
	nStations := 13
	if got != nStations {
		t.Errorf("asset.AvailableStations: %v => want %v", got, nStations)
	}
}

func TestSetNextFetchTime(t *testing.T) {
	// Test with nil NextFetchTime
	asset := &radikron.Asset{
		NextFetchTime: nil,
	}
	fixedTime := time.Date(2023, 6, 5, 13, 0, 0, 0, time.UTC)
	setNextFetchTime(asset, fixedTime)

	if asset.NextFetchTime == nil {
		t.Error("NextFetchTime should be set")
	}
	expected := fixedTime.Add(radikron.OneDay * time.Hour)
	if !asset.NextFetchTime.Equal(expected) {
		t.Errorf("NextFetchTime = %v, want %v", asset.NextFetchTime, expected)
	}

	// Test with existing NextFetchTime (should not change)
	existingTime := time.Date(2023, 6, 6, 13, 0, 0, 0, time.UTC)
	asset.NextFetchTime = &existingTime
	setNextFetchTime(asset, fixedTime)

	if !asset.NextFetchTime.Equal(existingTime) {
		t.Errorf("NextFetchTime should not change when already set, got %v, want %v", asset.NextFetchTime, existingTime)
	}
}

func TestProcessStation_SkipWhenNoRules(t *testing.T) {
	ctx := context.Background()
	wg := &sync.WaitGroup{}
	stationID := testStationID
	rules := radikron.Rules{}

	mockFetcher := &mockProgramFetcher{}
	mockDownloader := &mockDownloader{}

	// Should skip early when no rules match
	processStation(ctx, wg, stationID, rules, mockFetcher, mockDownloader)

	if mockFetcher.Called() {
		t.Error("FetchWeeklyPrograms should not be called when no rules match")
	}
}

func TestProcessStation_WithMatchingRules(t *testing.T) {
	ctx := context.Background()
	wg := &sync.WaitGroup{}
	stationID := testStationID

	rule := &radikron.Rule{}
	rule.SetName("test-rule")
	rule.StationID = stationID
	rules := radikron.Rules{rule}

	mockFetcher := &mockProgramFetcher{
		progs: radikron.Progs{
			{
				StationID: stationID,
				Title:     "Test Program",
				Ft:        "20230605130000",
				To:        "20230605140000",
			},
		},
	}
	mockDownloader := &mockDownloader{}

	processStation(ctx, wg, stationID, rules, mockFetcher, mockDownloader)

	if !mockFetcher.Called() {
		t.Error("FetchWeeklyPrograms should be called when rules match")
	}
	if mockFetcher.StationID() != stationID {
		t.Errorf("FetchWeeklyPrograms called with stationID = %v, want %v", mockFetcher.StationID(), stationID)
	}
}

func TestProcessStations(t *testing.T) {
	ctx := context.Background()
	wg := &sync.WaitGroup{}

	asset := &radikron.Asset{
		AvailableStations: []string{"FMT", "TBS"},
	}

	// Create rules that match both stations
	rule1 := &radikron.Rule{}
	rule1.SetName("test-rule-1")
	rule1.StationID = "FMT"

	rule2 := &radikron.Rule{}
	rule2.SetName("test-rule-2")
	rule2.StationID = "TBS"

	rules := radikron.Rules{rule1, rule2}

	mockFetcher := &mockProgramFetcher{}
	mockDownloader := &mockDownloader{}

	processStations(ctx, wg, asset, rules, mockFetcher, mockDownloader)

	// Should process both stations that have matching rules
	if mockFetcher.CallCount() != 2 {
		t.Errorf("processStations should process stations with matching rules, got %d calls, want 2", mockFetcher.CallCount())
	}
}

func TestProcessStations_EmptyStations(t *testing.T) {
	ctx := context.Background()
	wg := &sync.WaitGroup{}

	asset := &radikron.Asset{
		AvailableStations: []string{},
	}

	rules := radikron.Rules{}
	mockFetcher := &mockProgramFetcher{}
	mockDownloader := &mockDownloader{}

	processStations(ctx, wg, asset, rules, mockFetcher, mockDownloader)

	// Should not process any stations
	if mockFetcher.CallCount() != 0 {
		t.Errorf("processStations should not process any stations when list is empty, got %d calls", mockFetcher.CallCount())
	}
}

func TestRunIteration(t *testing.T) {
	var err error
	radikron.Location, err = time.LoadLocation(radikron.TZTokyo)
	if err != nil {
		t.Fatalf("Failed to load location: %v", err)
	}

	client, err := radiko.New("")
	if err != nil {
		t.Fatalf("Failed to create radiko client: %v", err)
	}

	asset, err := radikron.NewAsset(client)
	if err != nil {
		t.Fatalf("Failed to create asset: %v", err)
	}

	ctx := context.WithValue(context.Background(), contextKey, asset)

	wg := &sync.WaitGroup{}
	configFileName := testConfigFile

	mockFetcher := &mockProgramFetcher{}
	mockDownloader := &mockDownloader{}
	fixedTime := time.Date(2023, 6, 5, 13, 0, 0, 0, time.UTC)
	timeProvider := func() time.Time { return fixedTime }
	timeSetter := defaultTimeSetter

	err = runIteration(ctx, wg, configFileName, mockFetcher, mockDownloader, timeProvider, timeSetter)
	if err != nil {
		t.Errorf("runIteration should not return error: %v", err)
	}

	// Verify NextFetchTime was set
	if asset.NextFetchTime == nil {
		t.Error("NextFetchTime should be set after runIteration")
	}
}

func TestRunIteration_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	wg := &sync.WaitGroup{}
	configFileName := nonexistentConfigFile

	mockFetcher := &mockProgramFetcher{}
	mockDownloader := &mockDownloader{}

	err := runIteration(ctx, wg, configFileName, mockFetcher, mockDownloader, time.Now, defaultTimeSetter)
	if err == nil {
		t.Error("runIteration should return error for invalid config file")
	}
}

func TestRunLoopIteration(t *testing.T) {
	var err error
	radikron.Location, err = time.LoadLocation(radikron.TZTokyo)
	if err != nil {
		t.Fatalf("Failed to load location: %v", err)
	}

	client, err := radiko.New("")
	if err != nil {
		t.Fatalf("Failed to create radiko client: %v", err)
	}

	wg := &sync.WaitGroup{}
	configFileName := testConfigFile

	mockFetcher := &mockProgramFetcher{}
	mockDownloader := &mockDownloader{}
	fixedTime := time.Date(2023, 6, 5, 13, 0, 0, 0, time.UTC)
	timeProvider := func() time.Time { return fixedTime }
	timeSetter := defaultTimeSetter

	asset, err := runLoopIteration(wg, configFileName, client, radikron.NewAsset, mockFetcher, mockDownloader, timeProvider, timeSetter)
	if err != nil {
		t.Errorf("runLoopIteration should not return error: %v", err)
	}

	if asset == nil {
		t.Fatal("runLoopIteration should return an asset")
	}

	// Verify NextFetchTime was set
	if asset.NextFetchTime == nil {
		t.Error("NextFetchTime should be set after runLoopIteration")
	}
}

func TestRunLoopIteration_AssetCreationError(t *testing.T) {
	wg := &sync.WaitGroup{}
	configFileName := testConfigFile

	mockFetcher := &mockProgramFetcher{}
	mockDownloader := &mockDownloader{}
	timeProvider := time.Now
	timeSetter := defaultTimeSetter

	// Use a nil client to cause asset creation to fail
	var nilClient *radiko.Client
	assetCreator := func(client *radiko.Client) (*radikron.Asset, error) {
		if client == nil {
			return nil, fmt.Errorf("client is nil")
		}
		return radikron.NewAsset(client)
	}

	asset, err := runLoopIteration(wg, configFileName, nilClient, assetCreator, mockFetcher, mockDownloader, timeProvider, timeSetter)
	if err == nil {
		t.Error("runLoopIteration should return error when asset creation fails")
	}
	if asset != nil {
		t.Error("runLoopIteration should return nil asset when error occurs")
	}
}

func TestProcessStation_FetchError(t *testing.T) {
	ctx := context.Background()
	wg := &sync.WaitGroup{}
	stationID := testStationID

	rule := &radikron.Rule{}
	rule.SetName("test-rule")
	rule.StationID = stationID
	rules := radikron.Rules{rule}

	mockFetcher := &mockProgramFetcher{
		err: fmt.Errorf("fetch error"),
	}
	mockDownloader := &mockDownloader{}

	// Should handle fetch error gracefully
	processStation(ctx, wg, stationID, rules, mockFetcher, mockDownloader)

	if !mockFetcher.Called() {
		t.Error("FetchWeeklyPrograms should be called")
	}
	if mockDownloader.Called() {
		t.Error("Download should not be called when fetch fails")
	}
}

func TestProcessStation_DownloadError(t *testing.T) {
	ctx := context.Background()
	wg := &sync.WaitGroup{}
	stationID := testStationID

	rule := &radikron.Rule{}
	rule.SetName("test-rule")
	rule.StationID = stationID
	rules := radikron.Rules{rule}

	mockFetcher := &mockProgramFetcher{
		progs: radikron.Progs{
			{
				StationID: stationID,
				Title:     "Test Program",
				Ft:        "20230605130000",
				To:        "20230605140000",
			},
		},
	}
	mockDownloader := &mockDownloader{
		err: fmt.Errorf("download error"),
	}

	// Should handle download error gracefully
	processStation(ctx, wg, stationID, rules, mockFetcher, mockDownloader)

	if !mockFetcher.Called() {
		t.Error("FetchWeeklyPrograms should be called")
	}
	if !mockDownloader.Called() {
		t.Error("Download should be called even if it fails")
	}
}

func TestReloadConfig_NoAssetInContext(t *testing.T) {
	ctx := context.Background() // No asset in context
	configFileName := testConfigFile

	_, err := reloadConfig(ctx, configFileName, time.Now, defaultTimeSetter)
	if err == nil {
		t.Error("reloadConfig should return error when asset not found in context")
	}
	if err != nil && err.Error() != "asset not found in context" {
		t.Errorf("Expected 'asset not found in context' error, got: %v", err)
	}
}

func TestReloadConfig_InvalidConfigFile(t *testing.T) {
	var err error
	radikron.Location, err = time.LoadLocation(radikron.TZTokyo)
	if err != nil {
		t.Fatalf("Failed to load location: %v", err)
	}

	client, err := radiko.New("")
	if err != nil {
		t.Fatalf("Failed to create radiko client: %v", err)
	}

	asset, err := radikron.NewAsset(client)
	if err != nil {
		t.Fatalf("Failed to create asset: %v", err)
	}

	ctx := context.WithValue(context.Background(), contextKey, asset)
	configFileName := nonexistentConfigFile

	_, err = reloadConfig(ctx, configFileName, time.Now, defaultTimeSetter)
	if err == nil {
		t.Error("reloadConfig should return error for invalid config file")
	}
}

func TestRunIteration_AssetNotFoundAfterReload(t *testing.T) {
	// Create context without asset
	ctx := context.Background()
	wg := &sync.WaitGroup{}
	configFileName := testConfigFile

	mockFetcher := &mockProgramFetcher{}
	mockDownloader := &mockDownloader{}

	// This should fail because reloadConfig will fail (no asset in context)
	err := runIteration(ctx, wg, configFileName, mockFetcher, mockDownloader, time.Now, defaultTimeSetter)
	if err == nil {
		t.Error("runIteration should return error when asset not found")
	}
}

func TestRunIteration_AssetNotFoundAfterConfig(t *testing.T) {
	var err error
	radikron.Location, err = time.LoadLocation(radikron.TZTokyo)
	if err != nil {
		t.Fatalf("Failed to load location: %v", err)
	}

	client, err := radiko.New("")
	if err != nil {
		t.Fatalf("Failed to create radiko client: %v", err)
	}

	_, err = radikron.NewAsset(client)
	if err != nil {
		t.Fatalf("Failed to create asset: %v", err)
	}

	wg := &sync.WaitGroup{}
	configFileName := testConfigFile

	mockFetcher := &mockProgramFetcher{}
	mockDownloader := &mockDownloader{}
	timeProvider := func() time.Time { return time.Date(2023, 6, 5, 13, 0, 0, 0, time.UTC) }
	timeSetter := defaultTimeSetter

	// Create context without asset to simulate the edge case where asset is not found
	ctx := context.Background()

	err = runIteration(ctx, wg, configFileName, mockFetcher, mockDownloader, timeProvider, timeSetter)
	if err == nil {
		t.Error("runIteration should return error when asset not found after config reload")
	}
}

func TestRun_WithDoneChannel(t *testing.T) {
	var err error
	radikron.Location, err = time.LoadLocation(radikron.TZTokyo)
	if err != nil {
		t.Fatalf("Failed to load location: %v", err)
	}

	client, err := radiko.New("")
	if err != nil {
		t.Fatalf("Failed to create radiko client: %v", err)
	}

	wg := &sync.WaitGroup{}
	configFileName := testConfigFile
	done := make(chan struct{})

	mockFetcher := &mockProgramFetcher{}
	mockDownloader := &mockDownloader{}
	timeProvider := func() time.Time { return time.Date(2023, 6, 5, 13, 0, 0, 0, time.UTC) }
	timeSetter := defaultTimeSetter

	// Close done channel immediately to stop the loop
	close(done)

	// Run should return immediately when done channel is closed
	err = run(wg, configFileName, client, radikron.NewAsset, mockFetcher, mockDownloader, timeProvider, timeSetter, done)
	if err != nil {
		t.Errorf("run should return nil when done channel is closed, got: %v", err)
	}
}

func TestRun_WithIterationError(t *testing.T) {
	var err error
	radikron.Location, err = time.LoadLocation(radikron.TZTokyo)
	if err != nil {
		t.Fatalf("Failed to load location: %v", err)
	}

	client, err := radiko.New("")
	if err != nil {
		t.Fatalf("Failed to create radiko client: %v", err)
	}

	wg := &sync.WaitGroup{}
	configFileName := nonexistentConfigFile // This will cause an error
	done := make(chan struct{})

	mockFetcher := &mockProgramFetcher{}
	mockDownloader := &mockDownloader{}
	timeProvider := func() time.Time { return time.Date(2023, 6, 5, 13, 0, 0, 0, time.UTC) }
	timeSetter := defaultTimeSetter

	// Run should return error when iteration fails
	err = run(wg, configFileName, client, radikron.NewAsset, mockFetcher, mockDownloader, timeProvider, timeSetter, done)
	if err == nil {
		t.Error("run should return error when iteration fails")
	}
}

func TestRun_WithTimer(t *testing.T) {
	var err error
	radikron.Location, err = time.LoadLocation(radikron.TZTokyo)
	if err != nil {
		t.Fatalf("Failed to load location: %v", err)
	}

	client, err := radiko.New("")
	if err != nil {
		t.Fatalf("Failed to create radiko client: %v", err)
	}

	wg := &sync.WaitGroup{}
	configFileName := testConfigFile
	done := make(chan struct{})

	mockFetcher := &mockProgramFetcher{}
	mockDownloader := &mockDownloader{}
	fixedTime := time.Date(2023, 6, 5, 13, 0, 0, 0, time.UTC)
	timeProvider := func() time.Time { return fixedTime }
	timeSetter := defaultTimeSetter

	// Run in goroutine and close done after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(done)
	}()

	err = run(wg, configFileName, client, radikron.NewAsset, mockFetcher, mockDownloader, timeProvider, timeSetter, done)
	if err != nil {
		t.Errorf("run should handle timer correctly, got error: %v", err)
	}
}

// Mock implementations for testing
type mockProgramFetcher struct {
	mu        sync.Mutex
	called    bool
	callCount int
	stationID string
	progs     radikron.Progs
	err       error
}

func (m *mockProgramFetcher) FetchWeeklyPrograms(stationID string) (radikron.Progs, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called = true
	m.callCount++
	m.stationID = stationID
	return m.progs, m.err
}

// Thread-safe getters
func (m *mockProgramFetcher) Called() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.called
}

func (m *mockProgramFetcher) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func (m *mockProgramFetcher) StationID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stationID
}

type mockDownloader struct {
	mu        sync.Mutex
	called    bool
	callCount int
	prog      *radikron.Prog
	err       error
}

func (m *mockDownloader) Download(_ context.Context, _ *sync.WaitGroup, prog *radikron.Prog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called = true
	m.callCount++
	m.prog = prog
	return m.err
}

// Thread-safe getters
func (m *mockDownloader) Called() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.called
}

func (m *mockDownloader) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func (m *mockDownloader) Prog() *radikron.Prog {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.prog
}
