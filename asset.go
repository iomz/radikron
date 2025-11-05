package radikron

import (
	"context"
	cr "crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/yyoshiki41/go-radiko"
	"github.com/yyoshiki41/radigo"
)

var (
	// Base64FullKey holds the /assets/flutter_assets/assets/key/android.jpg in the v8 APK
	//go:embed assets/base64-full.key
	Base64FullKey embed.FS
	// CoordinatesJSON is a JSON contains the base GPS locations
	//go:embed assets/coordinates.json
	CoordinatesJSON embed.FS
	// RegionsJSON is a JSON contains the region mapping
	//go:embed assets/regions.json
	RegionsJSON embed.FS
	// VersionsJSON is a JSON contains the valid SDK versions
	//go:embed assets/versions.json
	VersionsJSON embed.FS
)

type Area struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Asset struct {
	AvailableStations []string
	AreaDevices       Devices
	Base64Key         string
	Coordinates       Coordinates
	DefaultClient     *radiko.Client
	// MinimumOutputSize in bytes for the downloaded audio
	MinimumOutputSize int64
	NextFetchTime     *time.Time
	OutputFormat      string
	Regions           Regions
	Rules             Rules
	Schedules         Schedules
	Stations          Stations
	Versions          Versions
}

// AddExtraStations appends stations to AvailableStations
func (a *Asset) AddExtraStations(es []string) {
	for _, s := range es {
		dup := false
		for _, as := range a.AvailableStations {
			if as == s {
				dup = true
				break
			}
		}
		if !dup {
			a.AvailableStations = append(a.AvailableStations, s)
		}
	}
}

// GenerateGPS returns the RadikoLocationHeader GPS string
// e.g., "35.689492,139.691701,gps"
func (a *Asset) GenerateGPSForAreaID(areaID string) string {
	if c, ok := a.Coordinates[areaID]; ok {
		negpos := []float64{-1.0, 1.0}
		// +/- 0 ~ 0.025 --> 0 ~ 1.5' -> +/- 0 ~ 2.77/2.13km
		rand.Shuffle(len(negpos), func(i, j int) {
			negpos[i], negpos[j] = negpos[j], negpos[i]
		})
		lat := c.Lat + (rand.Float64()/JapanLatLng)*negpos[0] //nolint:gosec
		rand.Shuffle(len(negpos), func(i, j int) {
			negpos[i], negpos[j] = negpos[j], negpos[i]
		})
		lng := c.Lng + (rand.Float64()/JapanLatLng)*negpos[0] //nolint:gosec
		return fmt.Sprintf("%.6f,%.6f,gps", lat, lng)
	}
	return ""
}

// GetAreaIDByStationID returns the first AreaID for the station
func (a *Asset) GetAreaIDByStationID(stationID string) string {
	if s, ok := a.Stations[stationID]; ok {
		return s.Areas[0]
	}
	return ""
}

// GetPartialKey returns the partial key for auth2 API
func (a *Asset) GetPartialKey(offset, length int64) (string, error) {
	authKey, err := base64.StdEncoding.DecodeString(a.Base64Key)
	if err != nil {
		return "", err
	}
	partialKey := base64.StdEncoding.EncodeToString(authKey[offset : offset+length])
	return partialKey, nil
}

// GetStationIDsByAreaID returns a slice of StationIDs
func (a *Asset) GetStationIDsByAreaID(areaID string) []string {
	sids := []string{}
	for sid, ss := range a.Stations {
		for _, aid := range ss.Areas {
			if aid == areaID {
				sids = append(sids, sid)
			}
		}
	}
	return sids
}

// LoadAvailableStations loads up the avaialable stations
func (a *Asset) LoadAvailableStations(areaID string) {
	// AvailableStations
	a.AvailableStations = a.GetStationIDsByAreaID(areaID)
}

// NewDevice returns a pointer to a new authorized Device
func (a *Asset) NewDevice(areaID string) (*Device, error) {
	// generate userID
	blob := make([]byte, UserIDLength)
	if _, err := cr.Read(blob); err != nil {
		return &Device{}, err
	}
	userID := hex.EncodeToString(blob)

	// new Device placeholder
	device := &Device{
		AppName:    "aSmartPhone7a",
		Connection: "wifi",
		UserID:     userID,
	}

	// get an app version
	as := make([]string, len(a.Versions.Apps))
	copy(as, a.Versions.Apps)
	rand.Shuffle(len(as), func(i, j int) {
		as[i], as[j] = as[j], as[i]
	})
	device.AppVersion = as[0]

	// get an sdk version
	svs := reflect.ValueOf(a.Versions.SDKs).MapKeys()
	rand.Shuffle(len(svs), func(i, j int) {
		svs[i], svs[j] = svs[j], svs[i]
	})
	sdkVersion := svs[0]

	// get an sdk
	sdk := a.Versions.SDKs[sdkVersion.String()]

	// get a sdk build
	bs := make([]string, len(sdk.Builds))
	copy(bs, sdk.Builds)
	rand.Shuffle(len(bs), func(i, j int) {
		bs[i], bs[j] = bs[j], bs[i]
	})
	build := bs[0]

	// get a model
	ms := make([]string, len(a.Versions.Models))
	copy(ms, a.Versions.Models)
	rand.Shuffle(len(ms), func(i, j int) {
		ms[i], ms[j] = ms[j], ms[i]
	})
	model := ms[0]

	// assign the detail
	// Dalvik/2.1.0 (Linux; U; Android %SDK_VERSION%; %MODEL%/%BUILD%)
	device.UserAgent = fmt.Sprintf(
		"Dalvik/2.1.0 (Linux; U; Android %s; %s/%s)",
		sdkVersion,
		model,
		build,
	)
	//X-Radiko-Device: %SDK_ID%.%MODEL%
	device.Name = fmt.Sprintf("%s.%s", sdk.ID, model)

	// get token
	err := device.Auth(a, areaID)
	if err != nil {
		return device, err
	}

	// save the device for areaID
	a.AreaDevices[areaID] = device
	return device, nil
}

// RemoveIgnoreStations remove stations from AvailableStations
func (a *Asset) RemoveIgnoreStations(is []string) {
	for _, s := range is {
		remove := -1
		for i, as := range a.AvailableStations {
			if as == s {
				remove = i
				break
			}
		}
		if remove != -1 {
			a.AvailableStations = append(
				a.AvailableStations[:remove],
				a.AvailableStations[remove+1:]...)
		}
	}
}

// UnmarshalJSON loads up Coordinates with Regions
func (a *Asset) UnmarshalJSON(b []byte) error {
	var cs map[string][]float64
	if err := json.Unmarshal(b, &cs); err != nil {
		return err
	}

	a.Coordinates = Coordinates{}
	for areaName, latlng := range cs {
		for _, areas := range a.Regions {
			for _, area := range areas {
				if areaName == area.Name {
					a.Coordinates[area.ID] = &Coordinate{
						Lat: latlng[0],
						Lng: latlng[1],
					}
				}
			}
		}
	}
	return nil
}

type ContextKey string

type Coordinate struct {
	Lat float64
	Lng float64
}

type Coordinates map[string]*Coordinate

type Device struct {
	AppName    string
	AppVersion string
	AuthToken  string
	Connection string
	Name       string
	UserAgent  string
	UserID     string
}

func (d *Device) Auth(a *Asset, areaID string) error {
	client := a.DefaultClient
	// auth1
	req, _ := http.NewRequest("GET", "https://radiko.jp/v2/api/auth1", http.NoBody)
	req = req.WithContext(context.Background())
	headers := map[string]string{
		UserAgentHeader:        d.UserAgent,
		RadikoAppHeader:        d.AppName,
		RadikoAppVersionHeader: d.AppVersion,
		RadikoDeviceHeader:     d.Name,
		RadikoUserHeader:       d.UserID,
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// auth2
	d.AuthToken = resp.Header.Get(RadikoAuthTokenHeader)
	offset, err := strconv.ParseInt(resp.Header.Get(RadikoKeyOffsetHeader), 10, 64)
	if err != nil {
		return err
	}
	length, err := strconv.ParseInt(resp.Header.Get(RadikoKeyLengthHeader), 10, 64)
	if err != nil {
		return err
	}
	partialKey, err := a.GetPartialKey(offset, length)
	if err != nil {
		return err
	}
	location := a.GenerateGPSForAreaID(areaID)
	req, _ = http.NewRequest("GET", "https://radiko.jp/v2/api/auth2", http.NoBody)
	req = req.WithContext(context.Background())
	headers = map[string]string{
		UserAgentHeader:        d.UserAgent,
		RadikoAppHeader:        d.AppName,
		RadikoAppVersionHeader: d.AppVersion,
		RadikoDeviceHeader:     d.Name,
		RadikoAuthTokenHeader:  d.AuthToken,
		RadikoUserHeader:       d.UserID,
		RadikoLocationHeader:   location,
		RadikoConnectionHeader: d.Connection,
		RadikoPartialKeyHeader: partialKey,
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return err
	}
	defer resp.Body.Close()
	return nil
}

type Devices map[string]*Device

type Regions map[string][]Area

type Schedules []*Prog

func (ss Schedules) HasDuplicate(prog *Prog) bool {
	for _, s := range ss {
		if s.ID == prog.ID {
			return true
		}
	}
	return false
}

type SDK struct {
	ID     string   `json:"sdk"`
	Builds []string `json:"builds"`
}

type Station struct {
	Areas []string
	Name  string
	Ruby  string
}

type Stations map[string]*Station

type Versions struct {
	Apps   []string        `json:"apps"`
	Models []string        `json:"models"`
	SDKs   map[string]*SDK `json:"sdks"`
}

func GetAsset(ctx context.Context) *Asset {
	k := ContextKey("asset")
	asset, ok := ctx.Value(k).(*Asset)
	if !ok {
		return nil
	}
	return asset
}

func NewAsset(client *radiko.Client) (*Asset, error) {
	asset := &Asset{}
	// empty AreaDevices
	asset.AreaDevices = map[string]*Device{}
	// the base64 key
	blob, err := Base64FullKey.ReadFile("assets/base64-full.key")
	if err != nil {
		return asset, err
	}
	asset.Base64Key = string(blob)
	// default client
	asset.DefaultClient = client
	// empty FileFormat
	asset.OutputFormat = radigo.AudioFormatAAC
	// nil *time.Time
	asset.NextFetchTime = nil
	// empty Schedules
	asset.Schedules = Schedules{}

	// Region
	regionsJSON, err := RegionsJSON.Open("assets/regions.json")
	if err != nil {
		return asset, err
	}
	defer regionsJSON.Close()
	blob, err = io.ReadAll(regionsJSON)
	if err != nil {
		return asset, err
	}
	err = json.Unmarshal(blob, &asset.Regions)
	if err != nil {
		return asset, err
	}

	// Coordinate
	coordinatesJSON, err := CoordinatesJSON.Open("assets/coordinates.json")
	if err != nil {
		return asset, err
	}
	defer coordinatesJSON.Close()
	blob, _ = io.ReadAll(coordinatesJSON)
	err = json.Unmarshal(blob, asset)
	if err != nil {
		return asset, err
	}

	// Station
	xmlRegion, err := FetchXMLRegion()
	if err != nil {
		return asset, err
	}
	asset.Stations = Stations{}
	for _, xmlStations := range xmlRegion.Region {
		for _, xmlStation := range xmlStations.Stations {
			if station, ok := asset.Stations[xmlStation.ID]; ok {
				station.Areas = append(station.Areas, xmlStation.AreaID)
			} else {
				station := &Station{
					Areas: []string{xmlStation.AreaID},
					Name:  xmlStation.Name,
					Ruby:  xmlStation.Ruby,
				}
				asset.Stations[xmlStation.ID] = station
			}
		}
	}

	// Versions
	versionsJSON, err := VersionsJSON.Open("assets/versions.json")
	if err != nil {
		return asset, err
	}
	defer versionsJSON.Close()
	blob, _ = io.ReadAll(versionsJSON)
	err = json.Unmarshal(blob, &asset.Versions)
	if err != nil {
		return asset, err
	}

	return asset, nil
}
