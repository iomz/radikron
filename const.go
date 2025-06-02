package radicron

const (
	// BufferMinutes for fetching the playlist.m3u8 chunks
	BufferMinutes = 5
	// DatetimeLayout for time strings from radiko
	DatetimeLayout = "20060102150405"
	// DefaultArea for radiko are
	DefaultArea = "JP13"
	// RetryDelaySecond for initial delay
	DefaultInitialDelaySeconds = 60
	// DefaultInterval to fetch the programs
	DefaultInterval = "168h"
	// DefaultMinimumOutputSize
	DefaultMinimumOutputSize = 1
	// Environment Variable for RADICRON_HOME
	EnvRadicronHome = "RADICRON_HOME"
	// Language for ID3v2 tags
	ID3v2LangJPN = "jpn"
	// LatLng for Japan
	JapanLatLng = 40.0
	// Kilobytes for the metric bytes
	Kilobytes = 1024
	// DefaultMaxConcurrents
	MaxConcurrency = 64
	// MaxRetryAttempts for BackOffDelay
	MaxRetryAttempts = 8
	// OneDay is 24 hours
	OneDay = 24
	// OutputDatetimeLayout for downloaded files
	OutputDatetimeLayout = "2006-01-02-1504"
	// TZTokyo for time location
	TZTokyo = "Asia/Tokyo"
	// UserIDLength for user-id
	UserIDLength = 16

	// API endpoints
	// region full
	APIRegionFull    = "https://radiko.jp/v3/station/region/full.xml"
	APIPlaylistM3U8  = "https://radiko.jp/v2/api/ts/playlist.m3u8"
	APIWeeklyProgram = "https://radiko.jp/v3/program/station/weekly/%s.xml"

	// HTTP Headers
	// auth1 req
	UserAgentHeader        = "User-Agent"
	RadikoAreaIDHeader     = "X-Radiko-AreaId"
	RadikoAppHeader        = "X-Radiko-App"
	RadikoAppVersionHeader = "X-Radiko-App-Version"
	RadikoDeviceHeader     = "X-Radiko-Device"
	RadikoUserHeader       = "X-Radiko-User"
	// auth1 res
	RadikoAuthTokenHeader = "X-Radiko-AuthToken" //nolint:gosec
	RadikoKeyLentghHeader = "X-Radiko-KeyLength"
	RadikoKeyOffsetHeader = "X-Radiko-KeyOffset"
	// auth2 req
	RadikoConnectionHeader = "X-Radiko-Connection"
	RadikoLocationHeader   = "X-Radiko-Location"
	RadikoPartialKeyHeader = "X-Radiko-Partialkey"
)
