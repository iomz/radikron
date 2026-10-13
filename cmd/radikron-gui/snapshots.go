package main

import (
	"log"
	"sync"

	"github.com/iomz/radikron"
)

// snapshotFetchWorkers is how many stations' weekly programs are fetched at
// once.
const snapshotFetchWorkers = 4

// snapshotLoad is a fetch of weekly programs in progress.
type snapshotLoad struct {
	done chan struct{}
	// full is true when the fetch refreshes every station, not only the
	// missing ones.
	full bool
}

// loadProgramSnapshots fetches the weekly programs of the available stations
// and stores them in a.programSnapshots: every station when full is true,
// otherwise only stations without programs yet. A call made while another
// fetch runs waits for it instead of fetching the same programs again.
func (a *App) loadProgramSnapshots(fetch func(stationID string) (radikron.Progs, error), full bool) {
	for {
		a.mu.Lock()
		if a.asset == nil {
			a.mu.Unlock()
			return
		}
		if running := a.snapshotLoad; running != nil {
			a.mu.Unlock()
			<-running.done
			if running.full || !full {
				return
			}
			continue
		}
		stations := make([]string, 0, len(a.asset.AvailableStations))
		for _, stationID := range a.asset.AvailableStations {
			if _, loaded := a.programSnapshots[stationID]; full || !loaded {
				stations = append(stations, stationID)
			}
		}
		if len(stations) == 0 {
			a.mu.Unlock()
			return
		}
		load := &snapshotLoad{done: make(chan struct{}), full: full}
		a.snapshotLoad = load
		a.mu.Unlock()

		a.fetchSnapshots(fetch, stations)

		a.mu.Lock()
		a.snapshotLoad = nil
		a.mu.Unlock()
		close(load.done)
		return
	}
}

// fetchSnapshots fetches stations a few at a time. A station that fails
// keeps its previous programs.
func (a *App) fetchSnapshots(fetch func(stationID string) (radikron.Progs, error), stations []string) {
	jobs := make(chan string)
	var wg sync.WaitGroup
	for range min(snapshotFetchWorkers, len(stations)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for stationID := range jobs {
				programs, err := fetch(stationID)
				if err != nil {
					log.Printf("failed to fetch the %s program: %v", stationID, err)
					continue
				}
				a.mu.Lock()
				a.programSnapshots[stationID] = programs
				a.mu.Unlock()
			}
		}()
	}
	for _, stationID := range stations {
		jobs <- stationID
	}
	close(jobs)
	wg.Wait()
}
