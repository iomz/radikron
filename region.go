package radikron

import (
	"encoding/xml"
	"io"
	"net/http"
)

type XMLRegion struct {
	Region []XMLRegionStations `xml:"stations"`
}

type XMLRegionStations struct {
	Stations   []XMLRegionStation `xml:"station"`
	RegionID   string             `xml:"region_id,attr"`
	RegionName string             `xml:"region_name,attr"`
}

type XMLRegionStation struct {
	ID     string `xml:"id"`
	Name   string `xml:"name"`
	AreaID string `xml:"area_id"`
	Ruby   string `xml:"ruby"`
}

func FetchXMLRegion() (XMLRegion, error) {
	region := XMLRegion{}

	resp, err := http.Get(APIRegionFull) //nolint:noctx
	if err != nil {
		return region, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return region, err
	}

	if err := xml.Unmarshal([]byte(string(body)), &region); err != nil {
		return region, err
	}

	return region, nil
}
