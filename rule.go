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

// FindMatch returns the first matching rule for the given station and program.
// Rules are evaluated in the order they appear in the configuration file, so an
// earlier rule takes precedence over a later one when both match.
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

// HasRuleWithCriteria returns true if at least one rule has optional criteria (Title, Pfm, or Keyword)
func (rs Rules) HasRuleWithCriteria() bool {
	for _, r := range rs {
		if r.HasTitle() || r.HasPfm() || r.HasKeyword() {
			return true
		}
	}
	return false
}

// Criteria is a set of program properties to test a program against.
//
// It is used twice in a Rule: once as the inclusion criteria, where every set
// property must match (conjunctive), and once as the optional exclusion
// criteria, where any single set property matching is enough to reject the
// program (disjunctive).
type Criteria struct {
	Title     string   `mapstructure:"title"`
	DoW       []string `mapstructure:"dow"`
	Keyword   string   `mapstructure:"keyword"`
	Pfm       string   `mapstructure:"pfm"`
	StationID string   `mapstructure:"station-id"`
}

type Rule struct {
	Name string `mapstructure:"name"` // required
	// Criteria is embedded so the inclusion properties stay flat in both the
	// config file and the Wails-generated frontend model.
	Criteria `mapstructure:",squash"`
	Exclude  *Criteria `mapstructure:"exclude"` // optional
	Window   string    `mapstructure:"window"`  // optional
	Folder   string    `mapstructure:"folder"`  // optional
}

// Match returns true if the rule matches the program
// 1. check the Window filter
// 2. check the DoW filter
// 3. check the StationID
// 4. match the criteria
// 5. reject when the exclusion criteria match
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
	// If no optional criteria (Title, Pfm, Keyword) are set, match nothing
	if !r.HasTitle() && !r.HasPfm() && !r.HasKeyword() {
		return false
	}
	if !r.matchPfm(p.Pfm, suppressLogs) || !r.matchTitle(p.Title, suppressLogs) || !r.matchKeyword(p, suppressLogs) {
		return false
	}

	// 5. reject when any exclusion criterion matches
	if reason, excluded := r.Exclude.hits(stationID, p); excluded {
		if !suppressLogs {
			log.Printf("rule[%s] excluded by %s", r.Name, reason)
		}
		return false
	}

	return true
}

// hits reports whether any single set criterion matches the program, along with
// a short description of the first one that did. A nil or empty Criteria never
// hits, so an absent or empty exclusion block is a no-op.
func (c *Criteria) hits(stationID string, p *Prog) (reason string, hit bool) {
	if c == nil {
		return "", false
	}
	if c.stationIDHit(stationID) {
		return "station-id: " + c.StationID, true
	}
	if c.dowHit(p.Ft) {
		return "dow: " + strings.Join(c.DoW, ","), true
	}
	if c.titleHit(p.Title) {
		return "title: '" + p.Title + "'", true
	}
	if c.pfmHit(p.Pfm) {
		return "pfm: '" + p.Pfm + "'", true
	}
	if field, ok := c.keywordHit(p); ok {
		return "keyword in " + field, true
	}
	return "", false
}

// The *Hit helpers report whether a criterion is set AND matches. The exported
// Match* methods below build the inclusion semantics on top of them, where an
// unset criterion matches everything.

func (c *Criteria) titleHit(title string) bool {
	return c.HasTitle() && strings.Contains(title, c.Title)
}

func (c *Criteria) pfmHit(pfm string) bool {
	return c.HasPfm() && strings.Contains(pfm, c.Pfm)
}

func (c *Criteria) stationIDHit(stationID string) bool {
	return c.HasStationID() && c.StationID == stationID
}

func (c *Criteria) dowHit(ft string) bool {
	if !c.HasDoW() {
		return false
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
	for _, d := range c.DoW {
		if st.Weekday() == dow[strings.ToLower(d)] {
			return true
		}
	}
	return false
}

// keywordHit reports whether the keyword is set and appears in any of the
// program's searchable fields, returning the name of the field that matched.
func (c *Criteria) keywordHit(p *Prog) (field string, hit bool) {
	if !c.HasKeyword() {
		return "", false
	}
	switch {
	case strings.Contains(p.Title, c.Keyword):
		return "title: '" + p.Title + "'", true
	case strings.Contains(p.Pfm, c.Keyword):
		return "pfm: '" + p.Pfm + "'", true
	case strings.Contains(p.Info, c.Keyword):
		return "info: " + strings.ReplaceAll(p.Info, "\n", ""), true
	case strings.Contains(p.Desc, c.Keyword):
		return "desc: '" + strings.ReplaceAll(p.Desc, "\n", "") + "'", true
	}
	for _, tag := range p.Tags {
		if strings.Contains(tag, c.Keyword) {
			return "tag: '" + tag + "'", true
		}
	}
	return "", false
}

func (c *Criteria) HasDoW() bool {
	return len(c.DoW) > 0
}

func (c *Criteria) HasPfm() bool {
	return c.Pfm != ""
}

func (c *Criteria) HasKeyword() bool {
	return c.Keyword != ""
}

func (c *Criteria) HasStationID() bool {
	if c.StationID == "" ||
		c.StationID == "*" {
		return false
	}
	return true
}

func (c *Criteria) HasTitle() bool {
	return c.Title != ""
}

func (r *Rule) HasWindow() bool {
	return r.Window != ""
}

// HasExclude reports whether the rule has any exclusion criterion set.
func (r *Rule) HasExclude() bool {
	if r.Exclude == nil {
		return false
	}
	return r.Exclude.HasTitle() || r.Exclude.HasPfm() || r.Exclude.HasKeyword() ||
		r.Exclude.HasDoW() || r.Exclude.HasStationID()
}

func (r *Rule) MatchDoW(ft string) bool {
	if !r.HasDoW() {
		return true
	}
	return r.dowHit(ft)
}

func (r *Rule) MatchKeyword(p *Prog) bool {
	return r.matchKeyword(p, false)
}

// matchKeyword is the internal keyword matching logic with optional log suppression
func (r *Rule) matchKeyword(p *Prog, suppressLogs bool) bool {
	if !r.HasKeyword() {
		return true // if no keyword, match all
	}
	field, ok := r.keywordHit(p)
	if !ok {
		return false
	}
	if !suppressLogs {
		log.Printf("rule[%s] matched with %s", r.Name, field)
	}
	return true
}

func (r *Rule) MatchPfm(pfm string) bool {
	return r.matchPfm(pfm, false)
}

// matchPfm is the internal pfm matching logic with optional log suppression
func (r *Rule) matchPfm(pfm string, suppressLogs bool) bool {
	if !r.HasPfm() {
		return true // if no pfm, match all
	}
	if !r.pfmHit(pfm) {
		return false
	}
	if !suppressLogs {
		log.Printf("rule[%s] matched with pfm: '%s'", r.Name, pfm)
	}
	return true
}

func (r *Rule) MatchStationID(stationID string) bool {
	if !r.HasStationID() {
		return true // if no station-id, match all
	}
	return r.stationIDHit(stationID)
}

func (r *Rule) MatchTitle(title string) bool {
	return r.matchTitle(title, false)
}

// matchTitle is the internal title matching logic with optional log suppression
func (r *Rule) matchTitle(title string, suppressLogs bool) bool {
	if !r.HasTitle() {
		return true // if no title, match all
	}
	if !r.titleHit(title) {
		return false
	}
	if !suppressLogs {
		log.Printf("rule[%s] matched with title: '%s'", r.Name, title)
	}
	return true
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
