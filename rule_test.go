package radikron

import (
	"testing"
	"time"
)

var matchtests = []struct {
	in        *Rule
	stationID string
	p         *Prog
	out       bool
}{
	{
		&Rule{"matchtests", "Title", []string{}, "Keyword", "Pfm", "FMT", ""},
		"FMT",
		&Prog{
			"ID",
			"FMT",
			"20230625050000",
			"20230625060000",
			"Title",
			"Keyword",
			"",
			"Pfm",
			[]string{},
			ProgGenre{},
			"",
		},
		true,
	},
	{
		&Rule{"matchtests", "RadioProgram", []string{}, "Keyword", "Pfm", "FMT", ""},
		"FMT",
		&Prog{
			"ID",
			"FMT",
			"20230625050000",
			"20230625060000",
			"Title", // title doesn't match
			"Keyword",
			"",
			"Pfm",
			[]string{},
			ProgGenre{},
			"",
		},
		false,
	},
	{
		&Rule{"matchtests", "RadioProgram", []string{}, "", "Someone", "FMT", ""},
		"FMT",
		&Prog{
			"ID",
			"FMT",
			"20230625050000",
			"20230625060000",
			"RadioProgram",
			"",
			"",
			"Pfm", // Pfm doesn't match
			[]string{},
			ProgGenre{},
			"",
		},
		false,
	},
}

func TestMatch(t *testing.T) {
	for _, tt := range matchtests {
		got := tt.in.Match(tt.stationID, tt.p)
		if got != tt.out {
			t.Errorf("(%v).Match => %v, want %v", tt.in, got, tt.out)
		}
	}
}

var dowtests = []struct {
	in  *Rule
	ft  string
	out bool
}{
	{
		&Rule{"dowtests", "Title", []string{}, "Keyword", "Pfm", "StationID", "Window"},
		"20230625050000", // sun
		true,
	},
	{
		&Rule{"dowtests", "Title", []string{"sun"}, "Keyword", "Pfm", "StationID", "Window"},
		"20230625050000", // sun
		true,
	},
	{
		&Rule{"dowtests", "Title", []string{"mon", "tue"}, "Keyword", "Pfm", "StationID", "Window"},
		"20230625050000", // sun
		false,
	},
}

func TestMatchDoW(t *testing.T) {
	for _, tt := range dowtests {
		got := tt.in.MatchDoW(tt.ft)
		if got != tt.out {
			t.Errorf("(%v).MatchDoW => %v, want %v", tt.in, got, tt.out)
		}
	}
}

var keywordtests = []struct {
	in   *Rule
	prog *Prog
	out  bool
}{
	{
		&Rule{"keywordtests", "Title", []string{}, "", "Pfm", "StationID", "Window"},
		&Prog{
			"ID",
			"StationID",
			"Ft",
			"To",
			"Title",
			"Desc",
			"Info",
			"Pfm",
			[]string{},
			ProgGenre{},
			"",
		},
		true,
	},
	{
		&Rule{"keywordtests", "Title", []string{}, "Keyword", "Pfm", "StationID", "Window"},
		&Prog{
			"ID",
			"StationID",
			"Ft",
			"To",
			"Keyword", // match
			"Desc",
			"Info",
			"Pfm",
			[]string{},
			ProgGenre{},
			"",
		},
		true,
	},
	{
		&Rule{"keywordtests", "Title", []string{}, "Keyword", "Pfm", "StationID", "Window"},
		&Prog{
			"ID",
			"StationID",
			"Ft",
			"To",
			"Title",
			"Keyword", // match
			"Info",
			"Pfm",
			[]string{},
			ProgGenre{},
			"",
		},
		true,
	},
	{
		&Rule{"keywordtests", "Title", []string{}, "Keyword", "Pfm", "StationID", "Window"},
		&Prog{
			"ID",
			"StationID",
			"Ft",
			"To",
			"Title",
			"Desc",
			"Keyword", // match
			"Pfm",
			[]string{},
			ProgGenre{},
			"",
		},
		true,
	},
	{
		&Rule{"keywordtests", "Title", []string{}, "Keyword", "Pfm", "StationID", "Window"},
		&Prog{
			"test",
			"test",
			"test",
			"test",
			"test",
			"test",
			"test",
			"Keyword", // match
			[]string{},
			ProgGenre{},
			"",
		},
		true,
	},
	{
		&Rule{"keywordtests", "Title", []string{}, "Keyword", "Pfm", "StationID", "Window"},
		&Prog{
			"test",
			"test",
			"test",
			"test",
			"test",
			"test",
			"test",
			"test",
			[]string{"Keyword"}, // match
			ProgGenre{},
			"test",
		},
		true,
	},
	{
		&Rule{"keywordtests", "Title", []string{}, "Keyword", "Pfm", "StationID", "Window"},
		&Prog{
			"ID",
			"StationID",
			"Ft",
			"To",
			"Title",
			"Desc",
			"Info",
			"Pfm",
			[]string{},
			ProgGenre{},
			"",
		},
		false,
	},
}

func TestMatchKeyword(t *testing.T) {
	for _, tt := range keywordtests {
		got := tt.in.MatchKeyword(tt.prog)
		if got != tt.out {
			t.Errorf("(%v).MatchKeyword => %v, want %v", tt.in, got, tt.out)
		}
	}
}

var pfmtests = []struct {
	in  *Rule
	pfm string
	out bool
}{
	{
		&Rule{"pfmtests", "Title", []string{"sun"}, "Keyword", "", "StationID", "Window"},
		"Pfm",
		true,
	},
	{
		&Rule{"pfmtests", "", []string{}, "", "Pfm", "", ""},
		"Pfm",
		true,
	},
	{
		&Rule{"pfmtests", "", []string{}, "", "Pfm", "", ""},
		"Someone",
		false,
	},
}

func TestMatchPfm(t *testing.T) {
	for _, tt := range pfmtests {
		got := tt.in.MatchPfm(tt.pfm)
		if got != tt.out {
			t.Errorf("(%v).MatchPfm => %v, want %v", tt.in, got, tt.out)
		}
	}
}

var stationtests = []struct {
	in        *Rule
	stationID string
	out       bool
}{
	{
		&Rule{"stationtests", "Title", []string{"sun"}, "Keyword", "Pfm", "FMT", "Window"},
		"FMT",
		true,
	},
	{
		&Rule{"stationtests", "", []string{}, "", "", "", ""},
		"FMT",
		true,
	},
	{
		&Rule{"stationtests", "", []string{}, "", "", "FMT", ""},
		"TBS",
		false,
	},
}

func TestMatchStationID(t *testing.T) {
	for _, tt := range stationtests {
		got := tt.in.MatchStationID(tt.stationID)
		if got != tt.out {
			t.Errorf("(%v).MatchStationID => %v, want %v", tt.in, got, tt.out)
		}
	}
}

var titletests = []struct {
	in    *Rule
	title string
	out   bool
}{
	{
		&Rule{"titletests", "Title", []string{"sun"}, "Keyword", "Pfm", "FMT", "Window"},
		"Title",
		true,
	},
	{
		&Rule{"titletests", "", []string{}, "", "", "", ""},
		"Title",
		true,
	},
	{
		&Rule{"titletests", "Title", []string{}, "", "", "FMT", ""},
		"Radio",
		false,
	},
}

func TestMatchTitle(t *testing.T) {
	for _, tt := range titletests {
		got := tt.in.MatchTitle(tt.title)
		if got != tt.out {
			t.Errorf("(%v).MatchTitle => %v, want %v", tt.in, got, tt.out)
		}
	}
}

var windowtests = []struct {
	in  *Rule
	ft  string
	out bool
}{
	{
		&Rule{"windowtests", "Title", []string{"sun"}, "Keyword", "Pfm", "FMT", ""},
		"20230625050000",
		true,
	},
	{
		&Rule{"windowtests", "", []string{}, "", "", "", "24h"},
		time.Now().Add(-1 * time.Hour).Format("20060102150405"),
		true,
	},
	{
		&Rule{"windowtests", "", []string{}, "", "", "", "24h"},
		time.Now().Add(time.Duration(-48) * time.Hour).Format("20060102150405"),
		false,
	},
}

func TestMatchWindow(t *testing.T) {
	Location, _ = time.LoadLocation(TZTokyo)
	CurrentTime = time.Now().In(Location)

	for _, tt := range windowtests {
		got := tt.in.MatchWindow(tt.ft)
		if got != tt.out {
			t.Errorf("(%v).MatchWindow => %v, want %v", tt.in, got, tt.out)
		}
	}
}

var ruletests = []struct {
	in  *Rule
	out bool
}{
	{
		&Rule{"ruletests", "Title", []string{"sun"}, "Keyword", "Pfm", "StationID", "Window"},
		true,
	},
	{
		&Rule{"ruletests", "", []string{}, "", "", "", ""},
		false,
	},
}

func TestHasDoW(t *testing.T) {
	for _, tt := range ruletests {
		if tt.in.HasDoW() != tt.out {
			t.Errorf("(%v).HasDoW => %v, want %v", tt.in, tt.in.HasDoW(), tt.out)
		}
	}
}

func TestHasPfm(t *testing.T) {
	for _, tt := range ruletests {
		if tt.in.HasPfm() != tt.out {
			t.Errorf("(%v).HasPfm  => %v, want %v", tt.in, tt.in.HasPfm(), tt.out)
		}
	}
}

func TestHasKeyword(t *testing.T) {
	for _, tt := range ruletests {
		if tt.in.HasKeyword() != tt.out {
			t.Errorf("(%v).HasKeyword  => %v, want %v", tt.in, tt.in.HasKeyword(), tt.out)
		}
	}
}

func TestHasStationID(t *testing.T) {
	for _, tt := range ruletests {
		if tt.in.HasStationID() != tt.out {
			t.Errorf("(%v).HasStationID  => %v, want %v", tt.in, tt.in.HasStationID(), tt.out)
		}
	}
}

func TestHasTitle(t *testing.T) {
	for _, tt := range ruletests {
		if tt.in.HasTitle() != tt.out {
			t.Errorf("(%v).HasTitle  => %v, want %v", tt.in, tt.in.HasTitle(), tt.out)
		}
	}
}

func TestHasWindow(t *testing.T) {
	for _, tt := range ruletests {
		if tt.in.HasWindow() != tt.out {
			t.Errorf("(%v).HasWindow  => %v, want %v", tt.in, tt.in.HasWindow(), tt.out)
		}
	}
}

func TestSetName(t *testing.T) {
	r := &Rule{}
	r.SetName("Name")
	if r.Name != "Name" {
		t.Errorf("(%v)  => %v, want %v", r, r.Name, "Name")
	}
}

func TestHasRuleFor(t *testing.T) {
	var rulestests = []struct {
		in  Rules
		sid string
		out bool
	}{
		{
			Rules{
				&Rule{"rulestests", "Title", []string{}, "Keyword", "Pfm", "FMT", "Window"},
				&Rule{"rulestests", "Title", []string{}, "Keyword", "Pfm", "TBS", "Window"},
			},
			"FMT",
			true,
		},
		{
			Rules{
				&Rule{"rulestests", "Title", []string{}, "Keyword", "Pfm", "FMT", "Window"},
				&Rule{"rulestests", "Title", []string{}, "Keyword", "Pfm", "TBS", "Window"},
			},
			"MBS",
			false,
		},
	}
	for _, tt := range rulestests {
		res := tt.in.HasRuleForStationID(tt.sid)
		if tt.out != res {
			t.Errorf("(%v).HasRuleFor(\"%s\") => %v, want %v", tt.in, tt.sid, res, tt.out)
		}
	}
}

func TestHasRuleWithoutStationID(t *testing.T) {
	var hrwsitests = []struct {
		in  Rules
		out bool
	}{
		{
			Rules{
				&Rule{"hrwsitests", "Title", []string{}, "Keyword", "Pfm", "", "Window"},
				&Rule{"hrwsitests", "Title", []string{}, "Keyword", "Pfm", "TBS", "Window"},
			},
			true,
		},
		{
			Rules{
				&Rule{"hrwsitests", "Title", []string{}, "Keyword", "Pfm", "FMT", "Window"},
				&Rule{"hrwsitests", "Title", []string{}, "Keyword", "Pfm", "TBS", "Window"},
			},
			false,
		},
	}
	for _, tt := range hrwsitests {
		res := tt.in.HasRuleWithoutStationID()
		if tt.out != res {
			t.Errorf("(%v).HasRuleWithoutStationID() => %v, want %v", tt.in, res, tt.out)
		}
	}
}
