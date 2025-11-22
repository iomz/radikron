package radikron

import (
	"log"
	"strings"
	"time"
)

type Rules []*Rule

func (rs Rules) HasMatch(stationID string, p *Prog) bool {
	for _, r := range rs {
		if r.Match(stationID, p) {
			return true
		}
	}
	return false
}

// FindMatch returns the first matching rule for the given station and program
func (rs Rules) FindMatch(stationID string, p *Prog) *Rule {
	for _, r := range rs {
		if r.Match(stationID, p) {
			return r
		}
	}
	return nil
}

// FindMatchSilent returns the first matching rule without logging
// This is useful when checking for matches on programs that may be skipped
func (rs Rules) FindMatchSilent(stationID string, p *Prog) *Rule {
	for _, r := range rs {
		if r.MatchSilent(stationID, p) {
			return r
		}
	}
	return nil
}

func (rs Rules) HasRuleWithoutStationID() bool {
	for _, r := range rs {
		if !r.HasStationID() {
			return true
		}
	}
	return false
}

func (rs Rules) HasRuleForStationID(stationID string) bool {
	for _, r := range rs {
		if r.StationID == stationID {
			return true
		}
	}
	return false
}

type Rule struct {
	Name      string   `mapstructure:"name"`       // required
	Title     string   `mapstructure:"title"`      // required if pfm and keyword are unset
	DoW       []string `mapstructure:"dow"`        // optional
	Keyword   string   `mapstructure:"keyword"`    // optional
	Pfm       string   `mapstructure:"pfm"`        // optional
	StationID string   `mapstructure:"station-id"` // optional
	Window    string   `mapstructure:"window"`     // optional
	Folder    string   `mapstructure:"folder"`     // optional
}

// Match returns true if the rule matches the program
// 1. check the Window filter
// 2. check the DoW filter
// 3. check the StationID
// 4. match the criteria
func (r *Rule) Match(stationID string, p *Prog) bool {
	return r.match(stationID, p, false)
}

// MatchSilent returns true if the rule matches the program without logging
func (r *Rule) MatchSilent(stationID string, p *Prog) bool {
	return r.match(stationID, p, true)
}

// match is the internal matching logic with optional log suppression
func (r *Rule) match(stationID string, p *Prog, suppressLogs bool) bool {
	// 1. check Window
	if !r.MatchWindow(p.Ft) {
		return false
	}
	// 2. check dow
	if !r.MatchDoW(p.Ft) {
		return false
	}
	// 3. check station-id
	if !r.MatchStationID(stationID) {
		return false
	}

	// 4. match
	if r.matchPfm(p.Pfm, suppressLogs) && r.matchTitle(p.Title, suppressLogs) && r.matchKeyword(p, suppressLogs) {
		return true
	}
	return false
}

func (r *Rule) HasDoW() bool {
	return len(r.DoW) > 0
}

func (r *Rule) HasPfm() bool {
	return r.Pfm != ""
}

func (r *Rule) HasKeyword() bool {
	return r.Keyword != ""
}

func (r *Rule) HasStationID() bool {
	if r.StationID == "" ||
		r.StationID == "*" {
		return false
	}
	return true
}

func (r *Rule) HasTitle() bool {
	return r.Title != ""
}

func (r *Rule) HasWindow() bool {
	return r.Window != ""
}

func (r *Rule) MatchDoW(ft string) bool {
	if !r.HasDoW() {
		return true
	}
	dow := map[string]time.Weekday{
		"sun": time.Sunday,
		"mon": time.Monday,
		"tue": time.Tuesday,
		"wed": time.Wednesday,
		"thu": time.Thursday,
		"fri": time.Friday,
		"sat": time.Saturday,
	}
	st, _ := time.ParseInLocation(DatetimeLayout, ft, Location)
	for _, d := range r.DoW {
		if st.Weekday() == dow[strings.ToLower(d)] {
			return true
		}
	}
	return false
}

func (r *Rule) MatchKeyword(p *Prog) bool {
	return r.matchKeyword(p, false)
}

// matchKeyword is the internal keyword matching logic with optional log suppression
func (r *Rule) matchKeyword(p *Prog, suppressLogs bool) bool {
	if !r.HasKeyword() {
		return true // if no keyword, match all
	}

	if strings.Contains(p.Title, r.Keyword) {
		if !suppressLogs {
			log.Printf("rule[%s] matched with title: '%s'", r.Name, p.Title)
		}
		return true
	} else if strings.Contains(p.Pfm, r.Keyword) {
		if !suppressLogs {
			log.Printf("rule[%s] matched with pfm: '%s'", r.Name, p.Pfm)
		}
		return true
	} else if strings.Contains(p.Info, r.Keyword) {
		if !suppressLogs {
			log.Printf("rule[%s] matched with info: %s", r.Name, strings.ReplaceAll(p.Info, "\n", ""))
		}
		return true
	} else if strings.Contains(p.Desc, r.Keyword) {
		if !suppressLogs {
			log.Printf("rule[%s] matched with desc: '%s'", r.Name, strings.ReplaceAll(p.Desc, "\n", ""))
		}
		return true
	}
	for _, tag := range p.Tags {
		if strings.Contains(tag, r.Keyword) {
			if !suppressLogs {
				log.Printf("rule[%s] matched with tag: '%s'", r.Name, tag)
			}
			return true
		}
	}
	return false
}

func (r *Rule) MatchPfm(pfm string) bool {
	return r.matchPfm(pfm, false)
}

// matchPfm is the internal pfm matching logic with optional log suppression
func (r *Rule) matchPfm(pfm string, suppressLogs bool) bool {
	if !r.HasPfm() {
		return true // if no pfm, match all
	}
	if strings.Contains(pfm, r.Pfm) {
		if !suppressLogs {
			log.Printf("rule[%s] matched with pfm: '%s'", r.Name, pfm)
		}
		return true
	}
	return false
}

func (r *Rule) MatchStationID(stationID string) bool {
	if !r.HasStationID() {
		return true // if no station-id, match all
	}
	if r.StationID == stationID {
		return true
	}
	return false
}

func (r *Rule) MatchTitle(title string) bool {
	return r.matchTitle(title, false)
}

// matchTitle is the internal title matching logic with optional log suppression
func (r *Rule) matchTitle(title string, suppressLogs bool) bool {
	if !r.HasTitle() {
		return true // if no title, match all
	}
	if strings.Contains(title, r.Title) {
		if !suppressLogs {
			log.Printf("rule[%s] matched with title: '%s'", r.Name, title)
		}
		return true
	}
	return false
}

func (r *Rule) MatchWindow(ft string) bool {
	if !r.HasWindow() {
		return true
	}
	startTime, err := time.ParseInLocation(DatetimeLayout, ft, Location)
	if err != nil {
		log.Printf("invalid start time format '%s': %s", ft, err)
		return false
	}
	fetchWindow, err := time.ParseDuration(r.Window)
	if err != nil {
		log.Printf("parsing [%s].window failed: %v (using 24h)", r.Name, err)
		fetchWindow = time.Hour * OneDay
	}
	if startTime.Add(fetchWindow).Before(CurrentTime) {
		return false // skip the program outside the fetch window
	}

	return true
}

func (r *Rule) SetName(name string) {
	r.Name = name
}
