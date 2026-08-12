package radikron

import (
	"encoding/xml"
	"os"
	"slices"
	"testing"
)

func TestArchiveStationPolicy(t *testing.T) {
	unsupported := []string{
		"JOIK", "JOHK", "JOAK", "JOCK", "JOBK", "JOFK", "JOZK", "JOLK", "JOAK-FM",
	}
	for _, stationID := range unsupported {
		if SupportsArchive(stationID) {
			t.Errorf("SupportsArchive(%q) = true, want false", stationID)
		}
	}

	if len(archiveUnsupportedStations) != len(unsupported) {
		t.Errorf("unsupported station count = %d, want %d", len(archiveUnsupportedStations), len(unsupported))
	}
	for _, stationID := range []string{"TBS", "MBS", "RN1", "RN2", "UNKNOWN"} {
		if !SupportsArchive(stationID) {
			t.Errorf("SupportsArchive(%q) = false, want true", stationID)
		}
	}
}

func TestStationMetadataIngestionEnforcesArchivePolicy(t *testing.T) {
	data, err := os.ReadFile("test/station-region-test.xml")
	if err != nil {
		t.Fatalf("read station fixture: %v", err)
	}
	var region XMLRegion
	if err := xml.Unmarshal(data, &region); err != nil {
		t.Fatalf("parse station fixture: %v", err)
	}

	stations := stationsFromRegion(region)
	if len(stations) != 3 {
		t.Fatalf("supported station count = %d, want 3: %v", len(stations), stations)
	}
	for _, stationID := range []string{"TBS", "RN1", "RN2"} {
		if _, ok := stations[stationID]; !ok {
			t.Errorf("ingested stations missing %q", stationID)
		}
	}
	for stationID := range archiveUnsupportedStations {
		if _, ok := stations[stationID]; ok {
			t.Errorf("ingested stations contains unsupported %q", stationID)
		}
	}
}

func TestAvailableStationsEnforceArchivePolicy(t *testing.T) {
	asset := &Asset{
		Stations: Stations{
			"JOAK": {Areas: []string{"JP13"}},
			"RN1":  {Areas: []string{"JP13"}},
			"RN2":  {Areas: []string{"JP13"}},
			"TBS":  {Areas: []string{"JP13"}},
		},
	}

	asset.LoadAvailableStations("JP13")
	asset.AddExtraStations([]string{"JOBK", "MBS"})

	for _, stationID := range []string{"TBS", "RN1", "RN2", "MBS"} {
		if !slices.Contains(asset.AvailableStations, stationID) {
			t.Errorf("AvailableStations missing supported station %q: %v", stationID, asset.AvailableStations)
		}
	}
	for _, stationID := range []string{"JOAK", "JOBK"} {
		if slices.Contains(asset.AvailableStations, stationID) {
			t.Errorf("AvailableStations contains unsupported station %q: %v", stationID, asset.AvailableStations)
		}
	}
}
