package radicron

import (
	"testing"
)

func TestFetchXMLRegion(t *testing.T) {
	const nRegions = 8
	const nStations = 109

	region, err := FetchXMLRegion()
	if err != nil {
		t.Error("failed to fetch the full region list")
	}
	if len(region.Region) != nRegions {
		t.Errorf("failed to fetch all the regions (%v instead of %v)", len(region.Region), nRegions)
	}
	stationCount := 0
	for _, stations := range region.Region {
		for _, station := range stations.Stations {
			t.Logf("%v (%v) %v\n", stations.RegionID, station.AreaID, station.Name)
			stationCount++
		}
	}
	if stationCount < nStations {
		t.Errorf("failed to fetch all the stations (%v instead of %v)", stationCount, nStations)
	}
}
