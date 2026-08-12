package radikron

// archiveUnsupportedStations contains Radiko station IDs that do not provide
// downloadable archive/timeshift audio. Radiko's station metadata currently
// reports timefree=1 for these stations, so that field cannot enforce policy.
var archiveUnsupportedStations = map[string]struct{}{
	"JOAK":    {},
	"JOAK-FM": {},
	"JOBK":    {},
	"JOCK":    {},
	"JOFK":    {},
	"JOHK":    {},
	"JOIK":    {},
	"JOLK":    {},
	"JOZK":    {},
}

// SupportsArchive reports whether Radikron supports archive downloads for a
// station. Unknown station IDs remain allowed for forward compatibility.
func SupportsArchive(stationID string) bool {
	_, unsupported := archiveUnsupportedStations[stationID]
	return !unsupported
}
