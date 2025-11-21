package main

import (
	"context"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/iomz/radikron"
	"github.com/yyoshiki41/go-radiko"
	"github.com/yyoshiki41/radigo"
)

const testStationID = "FMT"

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
	ck := radikron.ContextKey("asset")
	asset, err := radikron.NewAsset(client)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), ck, asset)
	cfg, err := reloadConfig(ctx, "test/config-test.yml", time.Now)
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

	if mockFetcher.called {
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

	if !mockFetcher.called {
		t.Error("FetchWeeklyPrograms should be called when rules match")
	}
	if mockFetcher.stationID != stationID {
		t.Errorf("FetchWeeklyPrograms called with stationID = %v, want %v", mockFetcher.stationID, stationID)
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
	if mockFetcher.callCount != 2 {
		t.Errorf("processStations should process stations with matching rules, got %d calls, want 2", mockFetcher.callCount)
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
	if mockFetcher.callCount != 0 {
		t.Errorf("processStations should not process any stations when list is empty, got %d calls", mockFetcher.callCount)
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

	ck := radikron.ContextKey("asset")
	ctx := context.WithValue(context.Background(), ck, asset)

	wg := &sync.WaitGroup{}
	configFileName := "test/config-test.yml"

	mockFetcher := &mockProgramFetcher{}
	mockDownloader := &mockDownloader{}
	fixedTime := time.Date(2023, 6, 5, 13, 0, 0, 0, time.UTC)
	timeProvider := func() time.Time { return fixedTime }

	err = runIteration(ctx, wg, configFileName, mockFetcher, mockDownloader, timeProvider)
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
	configFileName := "nonexistent-config.yml"

	mockFetcher := &mockProgramFetcher{}
	mockDownloader := &mockDownloader{}

	err := runIteration(ctx, wg, configFileName, mockFetcher, mockDownloader, time.Now)
	if err == nil {
		t.Error("runIteration should return error for invalid config file")
	}
}

// Mock implementations for testing
type mockProgramFetcher struct {
	called    bool
	callCount int
	stationID string
	progs     radikron.Progs
	err       error
}

func (m *mockProgramFetcher) FetchWeeklyPrograms(stationID string) (radikron.Progs, error) {
	m.called = true
	m.callCount++
	m.stationID = stationID
	return m.progs, m.err
}

type mockDownloader struct {
	called    bool
	callCount int
	prog      *radikron.Prog
	err       error
}

func (m *mockDownloader) Download(_ context.Context, _ *sync.WaitGroup, prog *radikron.Prog) error {
	m.called = true
	m.callCount++
	m.prog = prog
	return m.err
}
