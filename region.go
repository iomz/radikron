package radikron

import (
	"context"
	"encoding/xml"
	"fmt"
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
	client, err := NewRadikoHTTPClient()
	if err != nil {
		return XMLRegion{}, err
	}
	return fetchXMLRegionWithClient(client, APIRegionFull)
}

func fetchXMLRegionWithClient(client *http.Client, endpoint string) (XMLRegion, error) {
	return fetchXMLRegionWithClientContext(context.Background(), client, endpoint)
}

func fetchXMLRegionWithClientContext(ctx context.Context, client *http.Client, endpoint string) (XMLRegion, error) {
	if client == nil {
		return XMLRegion{}, fmt.Errorf("HTTP client is nil")
	}
	region := XMLRegion{}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return region, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return region, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return region, fmt.Errorf("station catalog returned HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return region, err
	}

	if err := xml.Unmarshal([]byte(string(body)), &region); err != nil {
		return region, err
	}

	return region, nil
}
