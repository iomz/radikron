package radikron

import (
	"encoding/json"
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
		&Rule{Name: "matchtests", Criteria: Criteria{Title: "Title", DoW: []string{}, Keyword: "Keyword", Pfm: "Pfm", StationID: "FMT"}},
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
			"",
			"",
			false,
		},
		true,
	},
	{
		&Rule{Name: "matchtests", Criteria: Criteria{
			Title:     "RadioProgram",
			DoW:       []string{},
			Keyword:   "Keyword",
			Pfm:       "Pfm",
			StationID: "FMT",
		}},
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
			"",
			"",
			false,
		},
		false,
	},
	{
		&Rule{Name: "matchtests", Criteria: Criteria{Title: "RadioProgram", DoW: []string{}, Pfm: "Someone", StationID: "FMT"}},
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
			"",
			"",
			false,
		},
		false,
	},
	{
		// Rule with no optional criteria (Title, Pfm, Keyword) should not match
		&Rule{Name: "matchtests", Criteria: Criteria{DoW: []string{}, StationID: "FMT"}},
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
			"",
			"",
			false,
		},
		false,
	},
}

func TestMatch(t *testing.T) {
	Location, _ = time.LoadLocation(TZTokyo)
	CurrentTime = time.Now().In(Location)

	for _, tt := range matchtests {
		got := tt.in.Match(tt.stationID, tt.p)
		if got != tt.out {
			t.Errorf("(%v).Match => %v, want %v", tt.in, got, tt.out)
		}
	}

	// Test Match with window exclusion
	r := &Rule{Name: "matchtests", Criteria: Criteria{
		Title:     "Title",
		DoW:       []string{},
		Keyword:   "Keyword",
		Pfm:       "Pfm",
		StationID: "FMT",
	}, Window: "1h"}
	p := &Prog{
		"ID",
		"FMT",
		time.Now().Add(-2 * time.Hour).Format("20060102150405"),
		"20230625060000",
		"Title",
		"Keyword",
		"",
		"Pfm",
		[]string{},
		ProgGenre{},
		"",
		"",
		"",
		false,
	}
	if r.Match("FMT", p) {
		t.Error("Match should return false when window excludes the program")
	}

	// Test Match with DoW exclusion
	r2 := &Rule{Name: "matchtests", Criteria: Criteria{
		Title:     "Title",
		DoW:       []string{"mon"},
		Keyword:   "Keyword",
		Pfm:       "Pfm",
		StationID: "FMT",
	}}
	p2 := &Prog{
		"ID",
		"FMT",
		"20230625050000", // Sunday
		"20230625060000",
		"Title",
		"Keyword",
		"",
		"Pfm",
		[]string{},
		ProgGenre{},
		"",
		"",
		"",
		false,
	}
	if r2.Match("FMT", p2) {
		t.Error("Match should return false when DoW doesn't match")
	}

	// Test Match with station ID exclusion
	r3 := &Rule{Name: "matchtests", Criteria: Criteria{Title: "Title", DoW: []string{}, Keyword: "Keyword", Pfm: "Pfm", StationID: "TBS"}}
	if r3.Match("FMT", p2) {
		t.Error("Match should return false when station ID doesn't match")
	}
}

var dowtests = []struct {
	in  *Rule
	ft  string
	out bool
}{
	{
		&Rule{Name: "dowtests", Criteria: Criteria{
			Title:     "Title",
			DoW:       []string{},
			Keyword:   "Keyword",
			Pfm:       "Pfm",
			StationID: "StationID",
		}, Window: "Window"},
		"20230625050000", // sun
		true,
	},
	{
		&Rule{Name: "dowtests", Criteria: Criteria{
			Title:     "Title",
			DoW:       []string{"sun"},
			Keyword:   "Keyword",
			Pfm:       "Pfm",
			StationID: "StationID",
		}, Window: "Window"},
		"20230625050000", // sun
		true,
	},
	{
		&Rule{Name: "dowtests", Criteria: Criteria{
			Title:     "Title",
			DoW:       []string{"mon", "tue"},
			Keyword:   "Keyword",
			Pfm:       "Pfm",
			StationID: "StationID",
		}, Window: "Window"},
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
		&Rule{Name: "keywordtests", Criteria: Criteria{
			Title:     "Title",
			DoW:       []string{},
			Pfm:       "Pfm",
			StationID: "StationID",
		}, Window: "Window"},
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
			"",
			"",
			false,
		},
		true,
	},
	{
		&Rule{Name: "keywordtests", Criteria: Criteria{
			Title:     "Title",
			DoW:       []string{},
			Keyword:   "Keyword",
			Pfm:       "Pfm",
			StationID: "StationID",
		}, Window: "Window"},
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
			"",
			"",
			false,
		},
		true,
	},
	{
		&Rule{Name: "keywordtests", Criteria: Criteria{
			Title:     "Title",
			DoW:       []string{},
			Keyword:   "Keyword",
			Pfm:       "Pfm",
			StationID: "StationID",
		}, Window: "Window"},
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
			"",
			"",
			false,
		},
		true,
	},
	{
		&Rule{Name: "keywordtests", Criteria: Criteria{
			Title:     "Title",
			DoW:       []string{},
			Keyword:   "Keyword",
			Pfm:       "Pfm",
			StationID: "StationID",
		}, Window: "Window"},
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
			"",
			"",
			false,
		},
		true,
	},
	{
		&Rule{Name: "keywordtests", Criteria: Criteria{
			Title:     "Title",
			DoW:       []string{},
			Keyword:   "Keyword",
			Pfm:       "Pfm",
			StationID: "StationID",
		}, Window: "Window"},
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
			"",
			"",
			false,
		},
		true,
	},
	{
		&Rule{Name: "keywordtests", Criteria: Criteria{
			Title:     "Title",
			DoW:       []string{},
			Keyword:   "Keyword",
			Pfm:       "Pfm",
			StationID: "StationID",
		}, Window: "Window"},
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
			"",
			"",
			false,
		},
		true,
	},
	{
		&Rule{Name: "keywordtests", Criteria: Criteria{
			Title:     "Title",
			DoW:       []string{},
			Keyword:   "Keyword",
			Pfm:       "Pfm",
			StationID: "StationID",
		}, Window: "Window"},
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
			"",
			"",
			false,
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
		&Rule{Name: "pfmtests", Criteria: Criteria{
			Title:     "Title",
			DoW:       []string{"sun"},
			Keyword:   "Keyword",
			StationID: "StationID",
		}, Window: "Window"},
		"Pfm",
		true,
	},
	{
		&Rule{Name: "pfmtests", Criteria: Criteria{DoW: []string{}, Pfm: "Pfm"}},
		"Pfm",
		true,
	},
	{
		&Rule{Name: "pfmtests", Criteria: Criteria{DoW: []string{}, Pfm: "Pfm"}},
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
		&Rule{Name: "stationtests", Criteria: Criteria{
			Title:     "Title",
			DoW:       []string{"sun"},
			Keyword:   "Keyword",
			Pfm:       "Pfm",
			StationID: "FMT",
		}, Window: "Window"},
		"FMT",
		true,
	},
	{
		&Rule{Name: "stationtests", Criteria: Criteria{DoW: []string{}}},
		"FMT",
		true,
	},
	{
		&Rule{Name: "stationtests", Criteria: Criteria{DoW: []string{}, StationID: "FMT"}},
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
		&Rule{Name: "titletests", Criteria: Criteria{
			Title:     "Title",
			DoW:       []string{"sun"},
			Keyword:   "Keyword",
			Pfm:       "Pfm",
			StationID: "FMT",
		}, Window: "Window"},
		"Title",
		true,
	},
	{
		&Rule{Name: "titletests", Criteria: Criteria{DoW: []string{}}},
		"Title",
		true,
	},
	{
		&Rule{Name: "titletests", Criteria: Criteria{Title: "Title", DoW: []string{}, StationID: "FMT"}},
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
		&Rule{Name: "windowtests", Criteria: Criteria{
			Title:     "Title",
			DoW:       []string{"sun"},
			Keyword:   "Keyword",
			Pfm:       "Pfm",
			StationID: "FMT",
		}},
		"20230625050000",
		true,
	},
	{
		&Rule{Name: "windowtests", Criteria: Criteria{DoW: []string{}}, Window: "24h"},
		time.Now().Add(-1 * time.Hour).Format("20060102150405"),
		true,
	},
	{
		&Rule{Name: "windowtests", Criteria: Criteria{DoW: []string{}}, Window: "24h"},
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

	// Test with invalid time format
	r := &Rule{Name: "windowtests", Criteria: Criteria{
		Title:     "Title",
		DoW:       []string{},
		Keyword:   "Keyword",
		Pfm:       "Pfm",
		StationID: "FMT",
	}, Window: "24h"}
	got := r.MatchWindow("invalid-time")
	if got {
		t.Error("MatchWindow should return false for invalid time format")
	}

	// Test with invalid window duration
	r2 := &Rule{Name: "windowtests", Criteria: Criteria{
		Title:     "Title",
		DoW:       []string{},
		Keyword:   "Keyword",
		Pfm:       "Pfm",
		StationID: "FMT",
	}, Window: "invalid"}
	got = r2.MatchWindow(time.Now().Add(-1 * time.Hour).Format("20060102150405"))
	if !got {
		t.Error("MatchWindow should handle invalid window duration gracefully")
	}
}

var ruletests = []struct {
	in  *Rule
	out bool
}{
	{
		&Rule{Name: "ruletests", Criteria: Criteria{
			Title:     "Title",
			DoW:       []string{"sun"},
			Keyword:   "Keyword",
			Pfm:       "Pfm",
			StationID: "StationID",
		}, Window: "Window"},
		true,
	},
	{
		&Rule{Name: "ruletests", Criteria: Criteria{DoW: []string{}}},
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
				&Rule{Name: "rulestests", Criteria: Criteria{
					Title:     "Title",
					DoW:       []string{},
					Keyword:   "Keyword",
					Pfm:       "Pfm",
					StationID: "FMT",
				}, Window: "Window"},
				&Rule{Name: "rulestests", Criteria: Criteria{
					Title:     "Title",
					DoW:       []string{},
					Keyword:   "Keyword",
					Pfm:       "Pfm",
					StationID: "TBS",
				}, Window: "Window"},
			},
			"FMT",
			true,
		},
		{
			Rules{
				&Rule{Name: "rulestests", Criteria: Criteria{
					Title:     "Title",
					DoW:       []string{},
					Keyword:   "Keyword",
					Pfm:       "Pfm",
					StationID: "FMT",
				}, Window: "Window"},
				&Rule{Name: "rulestests", Criteria: Criteria{
					Title:     "Title",
					DoW:       []string{},
					Keyword:   "Keyword",
					Pfm:       "Pfm",
					StationID: "TBS",
				}, Window: "Window"},
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
				&Rule{Name: "hrwsitests", Criteria: Criteria{
					Title:   "Title",
					DoW:     []string{},
					Keyword: "Keyword",
					Pfm:     "Pfm",
				}, Window: "Window"},
				&Rule{Name: "hrwsitests", Criteria: Criteria{
					Title:     "Title",
					DoW:       []string{},
					Keyword:   "Keyword",
					Pfm:       "Pfm",
					StationID: "TBS",
				}, Window: "Window"},
			},
			true,
		},
		{
			Rules{
				&Rule{Name: "hrwsitests", Criteria: Criteria{
					Title:     "Title",
					DoW:       []string{},
					Keyword:   "Keyword",
					Pfm:       "Pfm",
					StationID: "FMT",
				}, Window: "Window"},
				&Rule{Name: "hrwsitests", Criteria: Criteria{
					Title:     "Title",
					DoW:       []string{},
					Keyword:   "Keyword",
					Pfm:       "Pfm",
					StationID: "TBS",
				}, Window: "Window"},
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

func TestHasMatch(t *testing.T) {
	Location, _ = time.LoadLocation(TZTokyo)
	CurrentTime = time.Now().In(Location)

	var hasmatchtests = []struct {
		rules     Rules
		stationID string
		prog      *Prog
		expected  bool
	}{
		{
			findMatchRules(),
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
				"",
				"",
				false,
			},
			true,
		},
		{
			findMatchRules(),
			"MBS",
			&Prog{
				"ID",
				"MBS",
				"20230625050000",
				"20230625060000",
				"Title",
				"Keyword",
				"",
				"Pfm",
				[]string{},
				ProgGenre{},
				"",
				"",
				"",
				false,
			},
			false,
		},
		{
			Rules{},
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
				"",
				"",
				false,
			},
			false,
		},
	}

	for _, tt := range hasmatchtests {
		got := tt.rules.HasMatch(tt.stationID, tt.prog)
		if got != tt.expected {
			t.Errorf("Rules.HasMatch(%s, %v) => %v, want %v", tt.stationID, tt.prog, got, tt.expected)
		}
	}
}

// findMatchRules returns the two-rule set shared by the TestFindMatch cases.
func findMatchRules() Rules {
	return Rules{
		&Rule{Name: "rule1", Criteria: Criteria{Title: "Title", DoW: []string{}, Keyword: "Keyword", Pfm: "Pfm", StationID: "FMT"}},
		&Rule{Name: "rule2", Criteria: Criteria{
			Title:     "OtherTitle",
			DoW:       []string{},
			Keyword:   "OtherKeyword",
			Pfm:       "OtherPfm",
			StationID: "TBS",
		}},
	}
}

func TestFindMatch(t *testing.T) {
	Location, _ = time.LoadLocation(TZTokyo)
	CurrentTime = time.Now().In(Location)

	var findmatchtests = []struct {
		rules     Rules
		stationID string
		prog      *Prog
		expected  *Rule
	}{
		{
			findMatchRules(),
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
				"",
				"",
				false,
			},
			&Rule{Name: "rule1", Criteria: Criteria{Title: "Title", DoW: []string{}, Keyword: "Keyword", Pfm: "Pfm", StationID: "FMT"}},
		},
		{
			findMatchRules(),
			"TBS",
			&Prog{
				"ID",
				"TBS",
				"20230625050000",
				"20230625060000",
				"OtherTitle",
				"OtherKeyword",
				"",
				"OtherPfm",
				[]string{},
				ProgGenre{},
				"",
				"",
				"",
				false,
			},
			&Rule{Name: "rule2", Criteria: Criteria{
				Title:     "OtherTitle",
				DoW:       []string{},
				Keyword:   "OtherKeyword",
				Pfm:       "OtherPfm",
				StationID: "TBS",
			}},
		},
		{
			findMatchRules(),
			"MBS",
			&Prog{
				"ID",
				"MBS",
				"20230625050000",
				"20230625060000",
				"Title",
				"Keyword",
				"",
				"Pfm",
				[]string{},
				ProgGenre{},
				"",
				"",
				"",
				false,
			},
			nil,
		},
	}

	for _, tt := range findmatchtests {
		got := tt.rules.FindMatch(tt.stationID, tt.prog)
		if tt.expected == nil {
			if got != nil {
				t.Errorf("Rules.FindMatch(%s, %v) => %v, want nil", tt.stationID, tt.prog, got)
			}
		} else {
			if got == nil {
				t.Errorf("Rules.FindMatch(%s, %v) => nil, want %v", tt.stationID, tt.prog, tt.expected)
			} else if got.Name != tt.expected.Name {
				t.Errorf("Rules.FindMatch(%s, %v) => rule with name %s, want %s", tt.stationID, tt.prog, got.Name, tt.expected.Name)
			}
		}
	}
}

func TestFindMatchSilent(t *testing.T) {
	Location, _ = time.LoadLocation(TZTokyo)
	CurrentTime = time.Now().In(Location)

	var findmatchsilenttests = []struct {
		rules     Rules
		stationID string
		prog      *Prog
		expected  *Rule
	}{
		{
			findMatchRules(),
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
				"",
				"",
				false,
			},
			&Rule{Name: "rule1", Criteria: Criteria{Title: "Title", DoW: []string{}, Keyword: "Keyword", Pfm: "Pfm", StationID: "FMT"}},
		},
		{
			findMatchRules(),
			"MBS",
			&Prog{
				"ID",
				"MBS",
				"20230625050000",
				"20230625060000",
				"Title",
				"Keyword",
				"",
				"Pfm",
				[]string{},
				ProgGenre{},
				"",
				"",
				"",
				false,
			},
			nil,
		},
		{
			Rules{},
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
				"",
				"",
				false,
			},
			nil,
		},
	}

	for _, tt := range findmatchsilenttests {
		got := tt.rules.FindMatchSilent(tt.stationID, tt.prog)
		if tt.expected == nil {
			if got != nil {
				t.Errorf("Rules.FindMatchSilent(%s, %v) => %v, want nil", tt.stationID, tt.prog, got)
			}
		} else {
			if got == nil {
				t.Errorf("Rules.FindMatchSilent(%s, %v) => nil, want %v", tt.stationID, tt.prog, tt.expected)
			} else if got.Name != tt.expected.Name {
				t.Errorf("Rules.FindMatchSilent(%s, %v) => rule with name %s, want %s", tt.stationID, tt.prog, got.Name, tt.expected.Name)
			}
		}
	}
}

func TestMatchSilent(t *testing.T) {
	Location, _ = time.LoadLocation(TZTokyo)
	CurrentTime = time.Now().In(Location)

	rule := &Rule{Name: "silenttest", Criteria: Criteria{Title: "Title", DoW: []string{}, Keyword: "Keyword", Pfm: "Pfm", StationID: "FMT"}}
	prog := &Prog{
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
		"",
		"",
		false,
	}

	// Test that MatchSilent works like Match but without logging
	got := rule.MatchSilent("FMT", prog)
	if !got {
		t.Error("MatchSilent should return true for matching rule")
	}

	// Test with non-matching rule
	rule2 := &Rule{Name: "silenttest", Criteria: Criteria{
		Title:     "OtherTitle",
		DoW:       []string{},
		Keyword:   "Keyword",
		Pfm:       "Pfm",
		StationID: "FMT",
	}}
	got = rule2.MatchSilent("FMT", prog)
	if got {
		t.Error("MatchSilent should return false for non-matching rule")
	}

	// Test with rule that has no criteria (should not match)
	rule3 := &Rule{Name: "silenttest", Criteria: Criteria{DoW: []string{}, StationID: "FMT"}}
	got = rule3.MatchSilent("FMT", prog)
	if got {
		t.Error("MatchSilent should return false for rule with no criteria")
	}
}

func TestHasRuleWithCriteria(t *testing.T) {
	var hrwctests = []struct {
		in  Rules
		out bool
	}{
		{
			// Empty rules
			Rules{},
			false,
		},
		{
			// Rule with Title only
			Rules{
				&Rule{Name: "test", Criteria: Criteria{Title: "Title", DoW: []string{}, StationID: "FMT"}},
			},
			true,
		},
		{
			// Rule with Pfm only
			Rules{
				&Rule{Name: "test", Criteria: Criteria{DoW: []string{}, Pfm: "Pfm", StationID: "FMT"}},
			},
			true,
		},
		{
			// Rule with Keyword only
			Rules{
				&Rule{Name: "test", Criteria: Criteria{DoW: []string{}, Keyword: "Keyword", StationID: "FMT"}},
			},
			true,
		},
		{
			// Rule with Title and Pfm
			Rules{
				&Rule{Name: "test", Criteria: Criteria{Title: "Title", DoW: []string{}, Pfm: "Pfm", StationID: "FMT"}},
			},
			true,
		},
		{
			// Rule with all criteria
			Rules{
				&Rule{Name: "test", Criteria: Criteria{Title: "Title", DoW: []string{}, Keyword: "Keyword", Pfm: "Pfm", StationID: "FMT"}},
			},
			true,
		},
		{
			// Rule with no criteria
			Rules{
				&Rule{Name: "test", Criteria: Criteria{DoW: []string{}, StationID: "FMT"}},
			},
			false,
		},
		{
			// Multiple rules, all without criteria
			Rules{
				&Rule{Name: "test1", Criteria: Criteria{DoW: []string{}, StationID: "FMT"}},
				&Rule{Name: "test2", Criteria: Criteria{DoW: []string{}, StationID: "TBS"}},
			},
			false,
		},
		{
			// Multiple rules, some with criteria
			Rules{
				&Rule{Name: "test1", Criteria: Criteria{DoW: []string{}, StationID: "FMT"}},
				&Rule{Name: "test2", Criteria: Criteria{Title: "Title", DoW: []string{}, StationID: "TBS"}},
			},
			true,
		},
		{
			// Multiple rules, all with criteria
			Rules{
				&Rule{Name: "test1", Criteria: Criteria{Title: "Title1", DoW: []string{}, StationID: "FMT"}},
				&Rule{Name: "test2", Criteria: Criteria{DoW: []string{}, Keyword: "Keyword", StationID: "TBS"}},
				&Rule{Name: "test3", Criteria: Criteria{DoW: []string{}, Pfm: "Pfm", StationID: "MBS"}},
			},
			true,
		},
	}

	for _, tt := range hrwctests {
		res := tt.in.HasRuleWithCriteria()
		if tt.out != res {
			t.Errorf("(%v).HasRuleWithCriteria() => %v, want %v", tt.in, res, tt.out)
		}
	}
}

// buildProg returns a program with the fields the rule criteria inspect.
func buildProg(title, pfm, ft string, tags []string) *Prog {
	return &Prog{
		Title: title,
		Pfm:   pfm,
		Ft:    ft,
		Tags:  tags,
	}
}

func TestExcludeRejectsMatchingProgram(t *testing.T) {
	// 20230626050000 is a Monday.
	monday := "20230626050000"
	saturday := "20230701050000"

	for _, tt := range []struct {
		name    string
		exclude *Criteria
		prog    *Prog
		want    bool // true when the rule still matches
	}{
		{
			name:    "nil exclude is a no-op",
			exclude: nil,
			prog:    buildProg("MIDDAY LOUNGE", "HOST", monday, nil),
			want:    true,
		},
		{
			name:    "empty exclude is a no-op",
			exclude: &Criteria{},
			prog:    buildProg("MIDDAY LOUNGE", "HOST", monday, nil),
			want:    true,
		},
		{
			name:    "pfm excludes",
			exclude: &Criteria{Pfm: "GUEST"},
			prog:    buildProg("MIDDAY LOUNGE", "GUEST", monday, nil),
			want:    false,
		},
		{
			name:    "pfm that does not match leaves the rule matching",
			exclude: &Criteria{Pfm: "GUEST"},
			prog:    buildProg("MIDDAY LOUNGE", "HOST", monday, nil),
			want:    true,
		},
		{
			name:    "dow excludes",
			exclude: &Criteria{DoW: []string{"sat"}},
			prog:    buildProg("MIDDAY LOUNGE", "HOST", saturday, nil),
			want:    false,
		},
		{
			name:    "dow on another day leaves the rule matching",
			exclude: &Criteria{DoW: []string{"sat"}},
			prog:    buildProg("MIDDAY LOUNGE", "HOST", monday, nil),
			want:    true,
		},
		{
			name:    "title excludes",
			exclude: &Criteria{Title: "SPECIAL"},
			prog:    buildProg("MIDDAY LOUNGE SPECIAL", "HOST", monday, nil),
			want:    false,
		},
		{
			name:    "keyword excludes via tags",
			exclude: &Criteria{Keyword: "rerun"},
			prog:    buildProg("MIDDAY LOUNGE", "HOST", monday, []string{"rerun"}),
			want:    false,
		},
		{
			name:    "station-id excludes",
			exclude: &Criteria{StationID: "FMJ"},
			prog:    buildProg("MIDDAY LOUNGE", "HOST", monday, nil),
			want:    false,
		},
		{
			name:    "any one of several criteria is enough to exclude",
			exclude: &Criteria{Pfm: "NOBODY", DoW: []string{"sat"}},
			prog:    buildProg("MIDDAY LOUNGE", "HOST", saturday, nil),
			want:    false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := &Rule{
				Name:     "midday",
				Criteria: Criteria{Title: "MIDDAY LOUNGE"},
				Exclude:  tt.exclude,
			}
			if got := r.MatchSilent("FMJ", tt.prog); got != tt.want {
				t.Errorf("MatchSilent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExcludeDoesNotWidenAMatch(t *testing.T) {
	// A program that never matched the inclusion criteria must stay unmatched
	// regardless of the exclusion block.
	r := &Rule{
		Name:     "midday",
		Criteria: Criteria{Title: "MIDDAY LOUNGE"},
		Exclude:  &Criteria{Pfm: "GUEST"},
	}
	p := buildProg("EVENING DRIVE", "HOST", "20230626050000", nil)
	if r.MatchSilent("FMJ", p) {
		t.Error("MatchSilent() = true for a program outside the inclusion criteria")
	}
}

func TestFindMatchReturnsFirstMatchingRule(t *testing.T) {
	// Both rules match; the earlier one must win, and it must stay stable when
	// the rules are declared in the opposite order.
	p := buildProg("MIDDAY LOUNGE", "HOST", "20230626050000", nil)

	ab := Rules{
		{Name: "a", Criteria: Criteria{Keyword: "LOUNGE"}, Folder: "a"},
		{Name: "b", Criteria: Criteria{Keyword: "MIDDAY"}, Folder: "b"},
	}
	ba := Rules{
		{Name: "b", Criteria: Criteria{Keyword: "MIDDAY"}, Folder: "b"},
		{Name: "a", Criteria: Criteria{Keyword: "LOUNGE"}, Folder: "a"},
	}

	for _, r := range ab {
		if !r.MatchSilent("FMJ", p) {
			t.Fatalf("precondition: rule %s does not match", r.Name)
		}
	}

	if got := ab.FindMatchSilent("FMJ", p); got == nil || got.Name != "a" {
		t.Errorf("FindMatchSilent() = %v, want rule a", got)
	}
	if got := ba.FindMatchSilent("FMJ", p); got == nil || got.Name != "b" {
		t.Errorf("FindMatchSilent() = %v, want rule b", got)
	}
}

func TestExcludeFallsThroughToLaterRule(t *testing.T) {
	// Excluding from an earlier rule lets a later rule pick the program up,
	// which is how a special edition is routed to a different folder.
	special := buildProg("MIDDAY LOUNGE SPECIAL", "HOST", "20230626050000", nil)
	regular := buildProg("MIDDAY LOUNGE", "HOST", "20230626050000", nil)

	rules := Rules{
		{
			Name:     "midday",
			Criteria: Criteria{Title: "MIDDAY LOUNGE"},
			Exclude:  &Criteria{Title: "SPECIAL"},
			Folder:   "MIDDAY LOUNGE",
		},
		{
			Name:     "midday-special",
			Criteria: Criteria{Title: "MIDDAY LOUNGE SPECIAL"},
			Folder:   "SPECIALS",
		},
	}

	if got := rules.FindMatchSilent("FMJ", special); got == nil || got.Folder != "SPECIALS" {
		t.Errorf("special: FindMatchSilent() = %v, want the SPECIALS rule", got)
	}
	if got := rules.FindMatchSilent("FMJ", regular); got == nil || got.Folder != "MIDDAY LOUNGE" {
		t.Errorf("regular: FindMatchSilent() = %v, want the MIDDAY LOUNGE rule", got)
	}
}

func TestHasExclude(t *testing.T) {
	for _, tt := range []struct {
		name string
		rule *Rule
		want bool
	}{
		{"nil", &Rule{Name: "r"}, false},
		{"empty", &Rule{Name: "r", Exclude: &Criteria{}}, false},
		{"title", &Rule{Name: "r", Exclude: &Criteria{Title: "x"}}, true},
		{"dow", &Rule{Name: "r", Exclude: &Criteria{DoW: []string{"sat"}}}, true},
		{"station-id", &Rule{Name: "r", Exclude: &Criteria{StationID: "FMJ"}}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rule.HasExclude(); got != tt.want {
				t.Errorf("HasExclude() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRuleJSONKeepsInclusionCriteriaFlat(t *testing.T) {
	// The Wails frontend reads rule.Title, rule.StationID and friends directly
	// off the marshaled rule. Embedding Criteria must not nest them.
	r := &Rule{
		Name:     "midday",
		Criteria: Criteria{Title: "MIDDAY LOUNGE", StationID: "FMJ"},
		Exclude:  &Criteria{Pfm: "GUEST"},
		Folder:   "MIDDAY LOUNGE",
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}
	if _, nested := m["Criteria"]; nested {
		t.Errorf("Criteria is nested in %s", b)
	}
	for _, k := range []string{"Name", "Title", "StationID", "Exclude", "Folder"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing top-level key %q in %s", k, b)
		}
	}
	exclude, ok := m["Exclude"].(map[string]any)
	if !ok {
		t.Fatalf("Exclude is not an object in %s", b)
	}
	if exclude["Pfm"] != "GUEST" {
		t.Errorf("Exclude.Pfm = %v, want GUEST", exclude["Pfm"])
	}
}

func TestDoWWithUnparseableStartTime(t *testing.T) {
	// The zero time.Time is a Monday. An unparseable Ft must not be treated as
	// Monday, in either direction.
	bad := buildProg("MIDDAY LOUNGE", "HOST", "not-a-timestamp", nil)

	t.Run("does not include", func(t *testing.T) {
		r := &Rule{
			Name:     "midday",
			Criteria: Criteria{Title: "MIDDAY LOUNGE", DoW: []string{"mon"}},
		}
		if r.MatchSilent("FMJ", bad) {
			t.Error("MatchSilent() = true for an unparseable start time")
		}
	})

	t.Run("does not exclude", func(t *testing.T) {
		r := &Rule{
			Name:     "midday",
			Criteria: Criteria{Title: "MIDDAY LOUNGE"},
			Exclude:  &Criteria{DoW: []string{"mon"}},
		}
		if !r.MatchSilent("FMJ", bad) {
			t.Error("MatchSilent() = false; an unparseable start time must not exclude")
		}
	})
}
