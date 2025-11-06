package radikron

import (
	"context"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/yyoshiki41/go-radiko"
)

func TestNewAsset(t *testing.T) {
	const nAreas = 47
	const nRegions = 7
	const nStations = 109
	client, err := radiko.New("")
	if err != nil {
		t.Error(err)
	}

	asset, err := NewAsset(client)
	if err != nil {
		t.Errorf("failed to parse the asset %s", err)
	}

	// Area
	if len(asset.Regions) != nRegions {
		t.Errorf("wrong number of regions (%v instead of %v)", len(asset.Regions), nRegions)
	}
	areaCount := 0
	for region := range asset.Regions {
		for range asset.Regions[region] {
			areaCount++
		}
	}
	if areaCount != nAreas {
		t.Errorf("wrong number of areas (%v instead of %v)", areaCount, nAreas)
	}

	// Coordinate
	if len(asset.Coordinates) != nAreas {
		t.Errorf("wrong number of coordinates (%v instead of %v)", len(asset.Coordinates), nAreas)
	}

	// Station
	if len(asset.Stations) != nStations {
		t.Errorf("wrong number of stations (%v instead of %v)", len(asset.Stations), nStations)
	}

	// Versions
	if len(asset.Versions.Apps) != 26 {
		t.Errorf("wrong number of apps (%v instead of %v)", len(asset.Versions.Apps), 26)
	}
	if len(asset.Versions.Models) != 241 {
		t.Errorf("wrong number of models (%v instead of %v)", len(asset.Versions.Models), 241)
	}
	if len(asset.Versions.SDKs) != 10 {
		t.Errorf("wrong number of sdks (%v instead of %v)", len(asset.Versions.SDKs), 10)
	}
}

func TestGenerateGPSForAreaID(t *testing.T) {
	client, err := radiko.New("")
	if err != nil {
		t.Error(err)
	}

	asset, _ := NewAsset(client)
	var gpstests = []struct {
		in  string
		out bool
	}{
		{
			"JP13",
			true,
		},
		{
			"NONEXISTENT",
			false,
		},
	}
	for _, tt := range gpstests {
		got := asset.GenerateGPSForAreaID(tt.in)
		if !tt.out && got != "" { // todo check gps
			t.Errorf("%v => want %v", got, tt.out)
		} else if tt.out {
			gps := strings.Split(got, ",")
			lat, _ := strconv.ParseFloat(gps[0], 64)
			lng, _ := strconv.ParseFloat(gps[1], 64)
			c := asset.Coordinates[tt.in]
			deltaLimit := 1.0 / 40.0
			if math.Abs(c.Lat-lat) > deltaLimit {
				t.Errorf("wrong lat: %v => want %v", lat, c.Lat)
			} else if math.Abs(c.Lng-lng) > deltaLimit {
				t.Errorf("wrong lng: %v => want %v", lng, c.Lng)
			}
		}
	}
}

func TestGetAreaIDByStationID(t *testing.T) {
	client, err := radiko.New("")
	if err != nil {
		t.Error(err)
	}

	asset, _ := NewAsset(client)
	var areatests = []struct {
		in  string
		out string
	}{
		{
			"TBS",
			"JP13",
		},
		{
			"MBS",
			"JP27",
		},
		{
			"NONEXISTENT",
			"",
		},
	}
	for _, tt := range areatests {
		got := asset.GetAreaIDByStationID(tt.in)
		if got != tt.out {
			t.Errorf("%v => want %v", got, tt.out)
		}
	}
}

func TestGetStationIDsByAreaID(t *testing.T) {
	client, err := radiko.New("")
	if err != nil {
		t.Error(err)
	}

	asset, _ := NewAsset(client)
	var stationtests = []struct {
		in  string
		out []string
	}{
		{
			"JP13",
			[]string{
				"FMJ",
				"FMT",
				"INT",
				"JOAK",
				"JOAK-FM",
				"JORF",
				"LFR",
				"QRR",
				"RN1",
				"RN2",
				"TBS",
			},
		},
		{
			"NONEXISTENT",
			[]string{},
		},
	}
	for _, tt := range stationtests {
		got := asset.GetStationIDsByAreaID(tt.in)
		less := func(a, b string) bool { return a < b }
		if !cmp.Equal(got, tt.out, cmpopts.SortSlices(less)) {
			t.Errorf("%v => want %v", got, tt.out)
		}
	}
}

func TestGetPartialKey(t *testing.T) {
	client, err := radiko.New("")
	if err != nil {
		t.Error(err)
	}

	asset, _ := NewAsset(client)
	partialKey, err := asset.GetPartialKey(128, 16)
	if err != nil {
		t.Error(err)
	}
	want := "hXL82UFnK/lqxRp3RUCtUw=="
	if partialKey != want {
		t.Errorf("partialKey %v => want %v", partialKey, want)
	}
}

func TestNewDevice(t *testing.T) {
	client, err := radiko.New("")
	if err != nil {
		t.Error(err)
	}

	a, _ := NewAsset(client)
	device, err := a.NewDevice(context.Background(), "JP13")

	if err != nil {
		t.Error(err)
	}

	if device.AppName != "aSmartPhone7a" {
		t.Errorf("%v => want %v", device.AppName, "aSmartPhone7a")
	}
	if device.Connection != "wifi" {
		t.Errorf("%v => want %v", device.Connection, "wifi")
	}
	var got string
	got = device.UserID
	if len(got) != 32 {
		t.Errorf("%v => want %v", len(got), 32)
	}
	got = device.AppVersion
	if m, _ := regexp.Match(`^7\.[2-5]\.[0-9]{1,2}$`, []byte(got)); !m {
		t.Errorf("invalid AppVersion: %v", got)
	}
	got = device.Name
	if m, _ := regexp.Match(`^[0-9]{2}\..*$`, []byte(got)); !m {
		t.Errorf("invalid Name: %v", got)
	}
	got = device.UserAgent
	if !strings.HasPrefix(got, "Dalvik/2.1.0") {
		t.Errorf("invalid UserAgent: %v", got)
	}
	got = device.AuthToken
	if got == "" {
		t.Errorf("invalid AuthToken: %v", got)
	}
}

func TestSchedules(t *testing.T) {
	ss := Schedules{
		&Prog{
			ID: "12345",
		},
	}
	p := &Prog{
		ID: "12345",
	}
	if !ss.HasDuplicate(p) {
		t.Errorf("hasDuplicate: %v", p)
	}
}

func TestSchedulesHasDuplicateFalse(t *testing.T) {
	ss := Schedules{
		&Prog{
			ID: "12345",
		},
	}
	p := &Prog{
		ID: "67890",
	}
	if ss.HasDuplicate(p) {
		t.Errorf("hasDuplicate: %v should be false", p)
	}
}

func TestSchedulesHasDuplicateEmpty(t *testing.T) {
	ss := Schedules{}
	p := &Prog{
		ID: "12345",
	}
	if ss.HasDuplicate(p) {
		t.Errorf("hasDuplicate: %v should be false for empty schedule", p)
	}
}

func TestAddExtraStations(t *testing.T) {
	client, err := radiko.New("")
	if err != nil {
		t.Error(err)
	}

	asset, _ := NewAsset(client)
	asset.AvailableStations = []string{"TBS", "MBS"}

	// Test adding new stations
	asset.AddExtraStations([]string{"FMT", "FMJ"})
	if len(asset.AvailableStations) != 4 {
		t.Errorf("expected 4 stations, got %d", len(asset.AvailableStations))
	}

	// Test adding duplicate stations (should not add)
	asset.AddExtraStations([]string{"TBS", "NEW"})
	if len(asset.AvailableStations) != 5 {
		t.Errorf("expected 5 stations, got %d", len(asset.AvailableStations))
	}

	// Test adding empty list
	asset.AddExtraStations([]string{})
	if len(asset.AvailableStations) != 5 {
		t.Errorf("expected 5 stations after empty add, got %d", len(asset.AvailableStations))
	}
}

func TestRemoveIgnoreStations(t *testing.T) {
	client, err := radiko.New("")
	if err != nil {
		t.Error(err)
	}

	asset, _ := NewAsset(client)
	asset.AvailableStations = []string{"TBS", "MBS", "FMT", "FMJ"}

	// Test removing existing stations
	asset.RemoveIgnoreStations([]string{"TBS", "MBS"})
	if len(asset.AvailableStations) != 2 {
		t.Errorf("expected 2 stations, got %d", len(asset.AvailableStations))
	}
	if asset.AvailableStations[0] != "FMT" || asset.AvailableStations[1] != "FMJ" {
		t.Errorf("unexpected stations: %v", asset.AvailableStations)
	}

	// Test removing non-existent stations
	asset.RemoveIgnoreStations([]string{"NONEXISTENT"})
	if len(asset.AvailableStations) != 2 {
		t.Errorf("expected 2 stations after removing non-existent, got %d", len(asset.AvailableStations))
	}

	// Test removing empty list
	asset.RemoveIgnoreStations([]string{})
	if len(asset.AvailableStations) != 2 {
		t.Errorf("expected 2 stations after empty remove, got %d", len(asset.AvailableStations))
	}
}

func TestLoadAvailableStations(t *testing.T) {
	client, err := radiko.New("")
	if err != nil {
		t.Error(err)
	}

	asset, _ := NewAsset(client)
	asset.LoadAvailableStations("JP13")

	expectedStations := []string{
		"FMJ", "FMT", "INT", "JOAK", "JOAK-FM", "JORF", "LFR", "QRR", "RN1", "RN2", "TBS",
	}
	less := func(a, b string) bool { return a < b }
	if !cmp.Equal(asset.AvailableStations, expectedStations, cmpopts.SortSlices(less)) {
		t.Errorf("expected stations %v, got %v", expectedStations, asset.AvailableStations)
	}

	// Test with non-existent area
	asset.LoadAvailableStations("NONEXISTENT")
	if len(asset.AvailableStations) != 0 {
		t.Errorf("expected 0 stations for non-existent area, got %d", len(asset.AvailableStations))
	}
}

func TestGetAsset(t *testing.T) {
	client, err := radiko.New("")
	if err != nil {
		t.Error(err)
	}

	asset, _ := NewAsset(client)

	// Test getting asset from context
	ctx := context.WithValue(context.Background(), ContextKey("asset"), asset)
	got := GetAsset(ctx)
	if got != asset {
		t.Errorf("expected asset from context, got %v", got)
	}

	// Test getting asset when not in context
	ctx2 := context.Background()
	got2 := GetAsset(ctx2)
	if got2 != nil {
		t.Errorf("expected nil when asset not in context, got %v", got2)
	}

	// Test getting asset when wrong type in context
	ctx3 := context.WithValue(context.Background(), ContextKey("asset"), "not an asset")
	got3 := GetAsset(ctx3)
	if got3 != nil {
		t.Errorf("expected nil when wrong type in context, got %v", got3)
	}
}

func TestGetPartialKeyError(t *testing.T) {
	client, err := radiko.New("")
	if err != nil {
		t.Error(err)
	}

	asset, _ := NewAsset(client)
	// Corrupt the base64 key
	asset.Base64Key = "invalid base64!!!"

	_, err = asset.GetPartialKey(0, 16)
	if err == nil {
		t.Error("expected error for invalid base64 key")
	}
}

func TestGetPartialKeyDifferentOffset(t *testing.T) {
	client, err := radiko.New("")
	if err != nil {
		t.Error(err)
	}

	asset, _ := NewAsset(client)
	// Test with a different valid offset
	partialKey, err := asset.GetPartialKey(0, 16)
	if err != nil {
		t.Error(err)
	}
	if partialKey == "" {
		t.Error("expected non-empty partial key")
	}
}

func TestAuthContextCancellation(t *testing.T) {
	client, err := radiko.New("")
	if err != nil {
		t.Error(err)
	}

	asset, _ := NewAsset(client)
	device := &Device{
		AppName:    "aSmartPhone7a",
		AppVersion: "7.2.0",
		Name:       "02.SM-A515F",
		UserAgent:  "Dalvik/2.1.0 (Linux; U; Android 11; SM-A515F/RZ8A210801M)",
		UserID:     "0123456789abcdef0123456789abcdef",
		Connection: "wifi",
	}

	// Test with canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = device.Auth(ctx, asset, "JP13")
	if err == nil {
		t.Error("expected error for canceled context")
	}
}

func TestAuthWithTimeout(t *testing.T) {
	client, err := radiko.New("")
	if err != nil {
		t.Error(err)
	}

	asset, _ := NewAsset(client)
	device := &Device{
		AppName:    "aSmartPhone7a",
		AppVersion: "7.2.0",
		Name:       "02.SM-A515F",
		UserAgent:  "Dalvik/2.1.0 (Linux; U; Android 11; SM-A515F/RZ8A210801M)",
		UserID:     "0123456789abcdef0123456789abcdef",
		Connection: "wifi",
	}

	// Test with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond) // Ensure timeout has passed

	err = device.Auth(ctx, asset, "JP13")
	if err == nil {
		t.Error("expected error for timed out context")
	}
}

func TestUnmarshalJSONError(t *testing.T) {
	client, err := radiko.New("")
	if err != nil {
		t.Error(err)
	}

	asset, _ := NewAsset(client)
	// Test with invalid JSON
	err = asset.UnmarshalJSON([]byte("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestUnmarshalJSONInvalidFormat(t *testing.T) {
	client, err := radiko.New("")
	if err != nil {
		t.Error(err)
	}

	asset, _ := NewAsset(client)
	// Test with JSON that doesn't match expected format
	err = asset.UnmarshalJSON([]byte(`{"not": "a map of arrays"}`))
	if err == nil {
		t.Error("expected error for invalid JSON format")
	}
}
