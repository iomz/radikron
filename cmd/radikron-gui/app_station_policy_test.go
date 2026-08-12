package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/iomz/radikron"
)

func TestGetAllStationsExcludesUnsupportedArchiveStations(t *testing.T) {
	app := NewApp()
	app.asset = &radikron.Asset{
		Stations: radikron.Stations{
			"JOAK": {Name: "NHK AM"},
			"RN1":  {Name: "Radio NIKKEI 1"},
			"TBS":  {Name: "TBS Radio"},
		},
	}

	stations, err := app.GetAllStations()
	if err != nil {
		t.Fatalf("GetAllStations() error: %v", err)
	}
	if slices.Contains(stations, "JOAK") {
		t.Errorf("GetAllStations() contains unsupported station: %v", stations)
	}
	for _, stationID := range []string{"RN1", "TBS"} {
		if !slices.Contains(stations, stationID) {
			t.Errorf("GetAllStations() missing supported station %q: %v", stationID, stations)
		}
	}
}

func TestInjectProgramRejectsUnsupportedArchiveStation(t *testing.T) {
	app := NewApp()
	app.asset = &radikron.Asset{}

	err := app.InjectProgram(&radikron.Prog{StationID: "JOAK-FM"}, "")
	if err == nil || !strings.Contains(err.Error(), "does not support Radiko archive/timeshift downloads") {
		t.Fatalf("InjectProgram() error = %v, want unsupported station error", err)
	}
	if len(app.asset.Schedules) != 0 {
		t.Errorf("InjectProgram() scheduled unsupported station: %v", app.asset.Schedules)
	}
}

func TestSearchAndSchedulesExcludeUnsupportedArchiveStations(t *testing.T) {
	app := NewApp()
	app.asset = &radikron.Asset{
		AvailableStations: []string{"TBS"},
		OutputFormat:      "aac",
		DownloadDir:       t.TempDir(),
		Schedules: radikron.Schedules{
			{StationID: "JOAK", Title: "NHK Show", Ft: "20260812120000"},
		},
	}
	app.programSnapshots = map[string]radikron.Progs{
		"JOAK": {{StationID: "JOAK", Title: "NHK Show"}},
		"TBS":  {{StationID: "TBS", Title: "Commercial Show"}},
	}

	programs, err := app.SearchWeeklyPrograms("Show", "", "", "")
	if err != nil {
		t.Fatalf("SearchWeeklyPrograms() error: %v", err)
	}
	if len(programs) != 1 || programs[0].StationID != "TBS" {
		t.Errorf("SearchWeeklyPrograms() = %v, want only TBS", programs)
	}

	schedules, err := app.GetSchedules()
	if err != nil {
		t.Fatalf("GetSchedules() error: %v", err)
	}
	if len(schedules) != 0 {
		t.Errorf("GetSchedules() contains unsupported schedule: %v", schedules)
	}
}
