// Package model holds the shared domain types. Every other internal package
// depends on it; it depends on nothing but the standard library.
package model

import "time"

// Offer is one job/offer row in the marketplace pipeline.
type Offer struct {
	ID        string
	Title     string
	Location  string
	Category  string
	Amount    string
	Budget    string
	Status    string // open | requested | process | done
	Signal    string // OK | Attention | Review
	Supplier  string
	Progress  int
	Attention string
	// Structured facets, used by search ranking. Category/Location stay as the
	// human-facing strings; these are the normalized versions.
	Trade        string
	Region       string
	CrewSize     int
	Start        time.Time
	Requirements []string // a1, insurance, certificates, hardware
	Languages    []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Updated renders UpdatedAt as a coarse "12 min ago" string for the UI.
func (o Offer) Updated() string { return Ago(o.UpdatedAt) }

// Ago formats a timestamp the way the pipeline cards want it.
func Ago(t time.Time) string {
	if t.IsZero() {
		return "just now"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return itoa(int(d.Minutes())) + " min ago"
	case d < 24*time.Hour:
		return itoa(int(d.Hours())) + " h ago"
	case d < 48*time.Hour:
		return "yesterday"
	default:
		return itoa(int(d.Hours()/24)) + " d ago"
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

type Metric struct {
	Label string
	Value string
	Note  string
}

type Perspective struct {
	Key        string
	Label      string
	Title      string
	Subtitle   string
	Quote      string
	Primary    string
	Secondary  string
	Stats      []Metric
	Workflow   []string
	Pain       []string
	ActionName string
}

type Module struct {
	Name   string
	Body   string
	Impact string
}

type RoadmapItem struct {
	Phase string
	Title string
	Body  string
}

// ChatMessage is one turn of an assistant conversation.
type ChatMessage struct {
	ID             int64
	ConversationID string
	Role           string // user | assistant | system
	Content        string
	Reasoning      string
	CreatedAt      time.Time
}

// Conversation groups chat messages for one visitor session.
type Conversation struct {
	ID        string
	Role      string // owner | executor — which perspective the visitor picked
	Route     string // page the conversation started on
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Feedback is one extracted opinion about the app itself. It is what feeds the
// development loop: the assistant mines it out of ordinary conversation.
type Feedback struct {
	ID             int64
	ConversationID string
	Kind           string // bug | confusion | request | praise
	Theme          string // short slug-ish cluster label
	Severity       int    // 1 (nit) .. 5 (blocker)
	Verbatim       string // the user's own words
	Requested      string // the concrete change implied
	Route          string
	Role           string
	Source         string // chat | widget
	CreatedAt      time.Time
	Status         string // new | triaged | shipped | rejected
}

// BacklogItem is an LLM-clustered group of feedback, ranked for the dev loop.
type BacklogItem struct {
	ID          int64
	Title       string
	Rationale   string
	Theme       string
	Kind        string
	Count       int
	AvgSeverity float64
	Score       float64
	Evidence    []string // verbatim quotes
	Status      string   // proposed | accepted | shipped | rejected
	UpdatedAt   time.Time
}

// --- who is here -----------------------------------------------------------

// Profile is what the app learns about a visitor during onboarding. It is the
// reason every other surface can be personal: the pipeline, the search ranking
// and the assistant all read it.
type Profile struct {
	ID           string // opaque id, carried in a cookie
	Role         string // owner (GU) | executor (SU) | unknown
	Company      string
	Contact      string
	Trades       []string // normalized trade slugs: electrical, sanitary, steel, interior, energy
	Regions      []string // "Munich, DE", "DACH", "NL"
	CrewSize     int
	Languages    []string
	Documents    []string // a1, insurance, tax, certificates
	Availability string   // free text: "from 1 Oct, 3 weeks"
	Notes        string   // anything the model heard that has no field yet
	Completeness int      // 0..100, computed from the fields above
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Known returns true once the profile is worth personalizing against.
func (p Profile) Known() bool { return p.Role != "" && p.Role != "unknown" && p.Completeness >= 40 }

// ProfileFields is the canonical field order onboarding walks through. It is
// exported so the onboarding prompt and the progress meter stay in sync.
var ProfileFields = []string{"role", "trades", "regions", "crewSize", "documents", "availability"}

// --- what they asked for ---------------------------------------------------

// Intent is a natural-language request parsed into something the store can
// query. The model produces it; deterministic code executes it.
type Intent struct {
	Raw        string
	Kind       string // find_offers | find_crews | post_job | onboard | help
	Trades     []string
	Regions    []string
	Statuses   []string
	Keywords   []string
	Documents  []string
	CrewSize   int
	Timeframe  string
	BudgetHint string
	Confidence float64 // 0..1, as reported by the parser
	Fallback   bool    // true when parsed mechanically because the model was unavailable
}

// Match is one search result with its fit score and the reasons behind it.
// Every number shown to a user must be explainable, so Why is not optional.
type Match struct {
	Offer Offer
	Fit   int      // 0..100
	Why   []string // short reasons, most important first
}

// SavedSearch lets a visitor keep an intent they run often.
type SavedSearch struct {
	ID        int64
	ProfileID string
	Label     string
	Query     string
	CreatedAt time.Time
}
