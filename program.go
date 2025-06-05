package radikron

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
)

// Prog contains the solicited program metadata
type Prog struct {
	ID        string
	StationID string
	Ft        string
	To        string
	Title     string
	Desc      string
	Info      string
	Pfm       string
	Tags      []string
	Genre     ProgGenre
	M3U8      string
}

type ProgGenre struct {
	Personality string
	Program     string
}

// Progs is a slice of Prog.
type Progs []*Prog

func (ps *Progs) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var xw XMLWeekly
	if err := d.DecodeElement(&xw, &start); err != nil {
		return err
	}

	stationID := xw.XMLStations.Station[0].StationID
	for _, p := range xw.XMLStations.Station[0].Progs.Prog {
		prog := &Prog{
			ID:        p.ID,
			StationID: stationID,
			Ft:        p.Ft,
			To:        p.To,
			Title:     p.Title,
			Desc:      p.Desc,
			Info:      p.Info,
			Pfm:       p.Pfm,
			M3U8:      "",
		}
		prog.Genre = ProgGenre{
			Personality: p.Genre.Personality.Name,
			Program:     p.Genre.Program.Name,
		}
		for _, t := range p.Tag.Item {
			prog.Tags = append(prog.Tags, t.Name)
		}
		*ps = append(*ps, prog)
	}

	return nil
}

// XMLProg contains the raw program metadata
type XMLProg struct {
	ID    string `xml:"id,attr"`
	Ft    string `xml:"ft,attr"`
	To    string `xml:"to,attr"`
	Title string `xml:"title"`
	Desc  string `xml:"desc"`
	Info  string `xml:"info"`
	Pfm   string `xml:"pfm"`
	Tag   struct {
		Item []XMLProgItem `xml:"item"`
	} `xml:"tag"`
	Genre struct {
		Personality XMLProgItem `xml:"personality"`
		Program     XMLProgItem `xml:"program"`
	} `xml:"genre"`
}

type XMLProgs struct {
	Date string     `xml:"date"`
	Prog []*XMLProg `xml:"prog"`
}

type XMLProgItem struct {
	ID   string `xml:"id,attr,omitempty"`
	Name string `xml:"name"`
}

type XMLWeekly struct {
	XMLName     xml.Name `xml:"radiko"`
	XMLStations struct {
		XMLName xml.Name           `xml:"stations"`
		Station []XMLWeeklyStation `xml:"station"`
	} `xml:"stations"`
}

type XMLWeeklyStation struct {
	StationID string   `xml:"id,attr"`
	Name      string   `xml:"name"`
	Progs     XMLProgs `xml:"progs"`
}

// FetchWeeklyPrograms returns the weekly programs.
func FetchWeeklyPrograms(stationID string) (Progs, error) {
	endpoint := fmt.Sprintf(APIWeeklyProgram, stationID)

	resp, err := http.Get(endpoint) //nolint:gosec,noctx
	if err != nil {
		return Progs{}, err
	}
	defer resp.Body.Close()

	return decodeWeeklyProgram(resp.Body)
}

func decodeWeeklyProgram(iorc io.ReadCloser) (Progs, error) {
	progs := Progs{}
	body, err := io.ReadAll(iorc)
	if err != nil {
		return progs, err
	}

	err = xml.Unmarshal([]byte(string(body)), &progs)
	return progs, err
}
