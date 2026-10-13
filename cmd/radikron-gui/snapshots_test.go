package main

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iomz/radikron"
)

func TestProgramSnapshotsAreFetchedOnceForConcurrentCallers(t *testing.T) {
	app := NewApp()
	app.asset = &radikron.Asset{AvailableStations: []string{"TBS", "QRR", "LFR", "FMT", "FMJ"}}
	var calls atomic.Int32
	release := make(chan struct{})
	fetch := func(stationID string) (radikron.Progs, error) {
		calls.Add(1)
		<-release
		if stationID == "" {
			return nil, errors.New("no station")
		}
		return radikron.Progs{{ID: stationID}}, nil
	}

	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app.loadProgramSnapshots(fetch, false)
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := calls.Load(); got != 5 {
		t.Errorf("fetched %d times for 5 stations, want 5: callers must share one fetch", got)
	}
	if len(app.programSnapshots) != 5 {
		t.Errorf("loaded %d stations, want 5", len(app.programSnapshots))
	}

	// Loading the missing stations fetches nothing now; a full load fetches all.
	app.loadProgramSnapshots(fetch, false)
	if got := calls.Load(); got != 5 {
		t.Errorf("a load with nothing missing fetched %d more", got-5)
	}
	app.loadProgramSnapshots(fetch, true)
	if got := calls.Load(); got != 10 {
		t.Errorf("a full load fetched %d stations, want 5", got-5)
	}
}
