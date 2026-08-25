// Package onboarding turns a first visit into a profile, by conversation.
//
// First principle: never ask a human to fill in a taxonomy the machine can
// infer. The visitor writes one sentence in their own words; the local model
// extracts whatever fields it can see; onboarding only asks about what is
// still genuinely unknown, one thing at a time — and never twice.
//
// A visitor can also correct a wrong inference in plain language ("no, we do
// sanitary, not electrical"). That runs through a second extraction mode that
// removes and replaces rather than merges. The confirmation card shown after
// every turn is the same editable profile panel a visitor can reach any time
// at /start/profile, so an inference is never more than one click from fixed.
package onboarding

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"mm-machine/internal/app"
	"mm-machine/internal/llm"
	"mm-machine/internal/model"
	"mm-machine/internal/store"
)

// CookieName carries the profile id. It is deliberately not a login: the app
// remembers a visitor without asking them to become an account first.
const CookieName = "mm_profile"

// Handler serves the onboarding thread.
type Handler struct {
	deps app.Deps
	tpl  *template.Template

	mu    sync.Mutex
	skips map[string]int // profile id -> consecutive turns that taught nothing
}

// Register wires the onboarding routes onto mux.
func Register(mux *http.ServeMux, deps app.Deps) *Handler {
	h := &Handler{
		deps:  deps,
		tpl:   template.Must(template.New("onboarding").Parse(threadHTML + profileHTML)),
		skips: map[string]int{},
	}
	mux.HandleFunc("/start", h.start)
	mux.HandleFunc("/start/turn", h.Turn)
	mux.HandleFunc("/start/stream", h.stream)
	mux.HandleFunc("/start/profile", h.profilePanel)
	mux.HandleFunc("/start/profile/edit", h.profileEdit)
	mux.HandleFunc("/start/finish", h.finish)
	mux.HandleFunc("/start/reset", h.reset)
	return h
}

func newID() string {
	var b [10]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Ensure returns the visitor's profile, creating one (and setting the cookie)
// on first contact.
func Ensure(w http.ResponseWriter, r *http.Request, deps app.Deps) (model.Profile, error) {
	if c, err := r.Cookie(CookieName); err == nil && c.Value != "" {
		p, err := deps.Store.GetProfile(r.Context(), c.Value)
		if err == nil {
			return p, nil
		}
	}
	p := model.Profile{ID: newID(), Role: "unknown", CreatedAt: time.Now()}
	saved, err := deps.Store.UpsertProfile(r.Context(), p)
	if err != nil {
		return model.Profile{}, err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    saved.ID,
		Path:     "/",
		MaxAge:   int((90 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return saved, nil
}

// Current returns the profile behind the request without creating one.
func Current(r *http.Request, deps app.Deps) (model.Profile, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return model.Profile{}, false
	}
	p, err := deps.Store.GetProfile(r.Context(), c.Value)
	if err != nil {
		return model.Profile{}, false
	}
	return p, true
}

// NextQuestion returns the question to ask next, and whether onboarding is
// done. The order follows model.ProfileFields so the progress meter is
// honest, and it never returns a question about a field the profile already
// knows.
func NextQuestion(p model.Profile) (string, bool) {
	for _, field := range model.ProfileFields {
		switch field {
		case "role":
			if p.Role == "" || p.Role == "unknown" {
				return "Are you looking for a crew, or looking for work?", false
			}
		case "trades":
			if len(p.Trades) == 0 {
				if p.Role == "executor" {
					return "What trades does your crew cover?", false
				}
				return "What kind of work do you need done?", false
			}
		case "regions":
			if len(p.Regions) == 0 {
				return "Which region — city or country?", false
			}
		case "crewSize":
			if p.CrewSize == 0 {
				if p.Role == "executor" {
					return "How many people can you field?", false
				}
				return "How many people do you need?", false
			}
		case "documents":
			if len(p.Documents) == 0 {
				return "Which papers are ready — A1, insurance, certificates?", false
			}
		case "availability":
			if strings.TrimSpace(p.Availability) == "" {
				return "When does it start, and for how long?", false
			}
		}
	}
	return "", true
}

// missingFields lists every ProfileFields entry still unfilled, in order, as
// short labels for the completeness meter.
func missingFields(p model.Profile) []string {
	var out []string
	for _, field := range model.ProfileFields {
		switch field {
		case "role":
			if p.Role == "" || p.Role == "unknown" {
				out = append(out, "role")
			}
		case "trades":
			if len(p.Trades) == 0 {
				out = append(out, "trades")
			}
		case "regions":
			if len(p.Regions) == 0 {
				out = append(out, "region")
			}
		case "crewSize":
			if p.CrewSize == 0 {
				out = append(out, "crew size")
			}
		case "documents":
			if len(p.Documents) == 0 {
				out = append(out, "documents")
			}
		case "availability":
			if strings.TrimSpace(p.Availability) == "" {
				out = append(out, "availability")
			}
		}
	}
	return out
}

var (
	knownTrades = []string{"electrical", "sanitary", "steel", "interior", "energy", "drywall", "hvac"}
	knownDocs   = []string{"a1", "insurance", "certificates", "tax"}
)

const extractionSchema = `{"role":"owner|executor|unknown","trades":["slug"],"regions":["City, CC"],` +
	`"crewSize":0,"languages":["de"],"documents":["a1|insurance|certificates|tax"],` +
	`"availability":"free text","company":"","notes":""}`

// Extract merges whatever the model can see in one message into the profile.
// Fields it cannot see must come back empty — inventing a trade is worse than
// asking one more question. A dense, well-written sentence should fill every
// field it touches in this single pass.
func Extract(ctx context.Context, deps app.Deps, p model.Profile, message string) model.Profile {
	if deps.LLM == nil {
		return mergeMechanical(p, message)
	}
	var got struct {
		Role         string   `json:"role"`
		Trades       []string `json:"trades"`
		Regions      []string `json:"regions"`
		CrewSize     int      `json:"crewSize"`
		Languages    []string `json:"languages"`
		Documents    []string `json:"documents"`
		Availability string   `json:"availability"`
		Company      string   `json:"company"`
		Notes        string   `json:"notes"`
	}
	req := llm.Request{
		MaxTokens:   1024,
		Temperature: 0.1,
		Messages: []llm.Message{
			{Role: "system", Content: "You extract structured facts from one message a construction professional wrote about themselves, " +
				"often a single dense sentence covering several facts at once — read all of it, fill every field it supports. " +
				"Only fill a field if the message actually says it. Leave everything else empty or zero. " +
				"role: owner = needs a crew (Generalunternehmer), executor = offers work (Subunternehmer); " +
				"a crew describing its own trades and availability without asking for help is almost always executor. " +
				"trades: lowercase slugs from electrical, sanitary, steel, interior, energy, drywall, hvac. " +
				"regions: \"City, CC\" (ISO country code) when the country can be inferred, otherwise just the city."},
			{Role: "user", Content: message},
		},
	}
	if err := deps.LLM.JSON(ctx, req, extractionSchema, &got); err != nil {
		log.Printf("onboarding: extraction failed, falling back: %v", err)
		return mergeMechanical(p, message)
	}
	if got.Role == "owner" || got.Role == "executor" {
		p.Role = got.Role
	}
	p.Trades = mergeList(p.Trades, got.Trades)
	p.Regions = mergeList(p.Regions, got.Regions)
	p.Languages = mergeList(p.Languages, got.Languages)
	p.Documents = mergeList(p.Documents, got.Documents)
	if got.CrewSize > 0 {
		p.CrewSize = got.CrewSize
	}
	if strings.TrimSpace(got.Availability) != "" {
		p.Availability = strings.TrimSpace(got.Availability)
	}
	if strings.TrimSpace(got.Company) != "" {
		p.Company = strings.TrimSpace(got.Company)
	}
	if n := strings.TrimSpace(got.Notes); n != "" && !strings.Contains(p.Notes, n) {
		p.Notes = strings.TrimSpace(p.Notes + "\n" + n)
	}
	return p
}

// mergeMechanical is the no-model path: crude keyword spotting, so onboarding
// still moves forward when the cluster is down.
func mergeMechanical(p model.Profile, message string) model.Profile {
	m := strings.ToLower(message)
	switch {
	case strings.Contains(m, "need") || strings.Contains(m, "hire") || strings.Contains(m, "looking for a crew"):
		if p.Role == "" || p.Role == "unknown" {
			p.Role = "owner"
		}
	case strings.Contains(m, "offer") || strings.Contains(m, "my crew") || strings.Contains(m, "we're") || strings.Contains(m, "we are") || strings.Contains(m, "looking for work"):
		if p.Role == "" || p.Role == "unknown" {
			p.Role = "executor"
		}
	}
	for _, trade := range knownTrades {
		if strings.Contains(m, trade) {
			p.Trades = mergeList(p.Trades, []string{trade})
		}
	}
	for _, doc := range knownDocs {
		if strings.Contains(m, doc) {
			p.Documents = mergeList(p.Documents, []string{doc})
		}
	}
	if p.Notes == "" {
		p.Notes = message
	}
	return p
}

// isCorrection guesses whether a message is fixing something already said,
// rather than adding something new. It only needs to be right often enough
// to route to the extraction mode that can remove and replace — a false
// negative just falls back to merge, which still lets the fact through.
func isCorrection(message string) bool {
	m := strings.ToLower(strings.TrimSpace(message)) + " "
	cues := []string{"no, ", "no ", "not ", "actually", "instead", "wrong", "mistake", "correction", "should be", "n't "}
	for _, cue := range cues {
		if strings.Contains(m, cue) {
			return true
		}
	}
	return false
}

const correctionSchema = `{"remove":{"trades":["slug"],"regions":["City, CC"],"documents":["a1|insurance|certificates|tax"],"languages":["de"]},` +
	`"add":{"trades":["slug"],"regions":["City, CC"],"documents":["a1|insurance|certificates|tax"],"languages":["de"]},` +
	`"role":"owner|executor|","crewSize":0,"availability":"","company":""}`

// ExtractCorrection reads one message as a fix to what the profile already
// holds: values named for removal come out, values named as their
// replacement go in. Unlike Extract, silence about a field means "leave it
// alone" — a correction only ever touches what the visitor is correcting.
func ExtractCorrection(ctx context.Context, deps app.Deps, p model.Profile, message string) model.Profile {
	if deps.LLM == nil {
		return mechanicalCorrection(p, message)
	}
	var got struct {
		Remove struct {
			Trades    []string `json:"trades"`
			Regions   []string `json:"regions"`
			Documents []string `json:"documents"`
			Languages []string `json:"languages"`
		} `json:"remove"`
		Add struct {
			Trades    []string `json:"trades"`
			Regions   []string `json:"regions"`
			Documents []string `json:"documents"`
			Languages []string `json:"languages"`
		} `json:"add"`
		Role         string `json:"role"`
		CrewSize     int    `json:"crewSize"`
		Availability string `json:"availability"`
		Company      string `json:"company"`
	}
	req := llm.Request{
		MaxTokens:   768,
		Temperature: 0.1,
		Messages: []llm.Message{
			{Role: "system", Content: "The visitor is correcting a fact they stated earlier, e.g. \"no, we do sanitary, not electrical\". " +
				"Put what must be removed under remove, and what replaces it under add. " +
				"Only include a field at all if this message is actually correcting it — do not restate facts that are not being changed. " +
				"role/crewSize/availability/company are single values: only set one if the visitor is replacing that exact fact. " +
				"trades must come from: " + strings.Join(knownTrades, ", ") + ". documents must come from: " + strings.Join(knownDocs, ", ") + "."},
			{Role: "user", Content: message},
		},
	}
	if err := deps.LLM.JSON(ctx, req, correctionSchema, &got); err != nil {
		log.Printf("onboarding: correction extraction failed, falling back: %v", err)
		return mechanicalCorrection(p, message)
	}
	p.Trades = removeThenAdd(p.Trades, got.Remove.Trades, got.Add.Trades)
	p.Regions = removeThenAdd(p.Regions, got.Remove.Regions, got.Add.Regions)
	p.Documents = removeThenAdd(p.Documents, got.Remove.Documents, got.Add.Documents)
	p.Languages = removeThenAdd(p.Languages, got.Remove.Languages, got.Add.Languages)
	if got.Role == "owner" || got.Role == "executor" {
		p.Role = got.Role
	}
	if got.CrewSize > 0 {
		p.CrewSize = got.CrewSize
	}
	if v := strings.TrimSpace(got.Availability); v != "" {
		p.Availability = v
	}
	if v := strings.TrimSpace(got.Company); v != "" {
		p.Company = v
	}
	return p
}

// mechanicalCorrection is the no-model correction path: it looks for
// "not <trade>" / "no <trade>" to remove, and any other known trade or
// document mentioned to add.
func mechanicalCorrection(p model.Profile, message string) model.Profile {
	m := strings.ToLower(message)
	for _, trade := range knownTrades {
		switch {
		case strings.Contains(m, "not "+trade), strings.Contains(m, "no "+trade):
			p.Trades = removeValue(p.Trades, trade)
		case strings.Contains(m, trade):
			p.Trades = mergeList(p.Trades, []string{trade})
		}
	}
	for _, doc := range knownDocs {
		switch {
		case strings.Contains(m, "not "+doc), strings.Contains(m, "no "+doc):
			p.Documents = removeValue(p.Documents, doc)
		case strings.Contains(m, doc):
			p.Documents = mergeList(p.Documents, []string{doc})
		}
	}
	return p
}

func mergeList(have, add []string) []string {
	for _, a := range add {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" {
			continue
		}
		found := false
		for _, h := range have {
			if strings.EqualFold(h, a) {
				found = true
				break
			}
		}
		if !found {
			have = append(have, a)
		}
	}
	return have
}

// removeValue drops every case-insensitive match of v from have.
func removeValue(have []string, v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return have
	}
	out := make([]string, 0, len(have))
	for _, h := range have {
		if !strings.EqualFold(strings.TrimSpace(h), v) {
			out = append(out, h)
		}
	}
	return out
}

// removeThenAdd applies a correction's remove list before its add list, so a
// value named in both (a same-turn swap) ends up added, not cancelled out.
func removeThenAdd(have, remove, add []string) []string {
	for _, r := range remove {
		have = removeValue(have, r)
	}
	return mergeList(have, add)
}

// factsEqual reports whether two profiles hold the same searchable facts,
// ignoring notes, company and timestamps. It is how a turn recognises that
// nothing new was actually learned.
func factsEqual(a, b model.Profile) bool {
	return a.Role == b.Role &&
		a.CrewSize == b.CrewSize &&
		strings.EqualFold(strings.TrimSpace(a.Availability), strings.TrimSpace(b.Availability)) &&
		sameSet(a.Trades, b.Trades) &&
		sameSet(a.Regions, b.Regions) &&
		sameSet(a.Documents, b.Documents) &&
		sameSet(a.Languages, b.Languages)
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, x := range a {
		found := false
		for _, y := range b {
			if strings.EqualFold(x, y) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// newItems returns entries of now that have with case-insensitive
// matching, in now's order — the part of a diff worth telling the visitor
// about.
func newItems(have, now []string) []string {
	var out []string
	for _, n := range now {
		found := false
		for _, h := range have {
			if strings.EqualFold(h, n) {
				found = true
				break
			}
		}
		if !found {
			out = append(out, n)
		}
	}
	return out
}

// describeLearned turns the diff between two profiles into a short, plain
// list of what changed this turn — the hint the streamed acknowledgement is
// built from.
func describeLearned(before, after model.Profile) string {
	var parts []string
	if before.Role != after.Role && after.Role != "" && after.Role != "unknown" {
		parts = append(parts, "role: "+after.Role)
	}
	if d := newItems(before.Trades, after.Trades); len(d) > 0 {
		parts = append(parts, "trades: "+strings.Join(d, ", "))
	}
	if d := newItems(before.Regions, after.Regions); len(d) > 0 {
		parts = append(parts, "region: "+strings.Join(d, ", "))
	}
	if before.CrewSize != after.CrewSize && after.CrewSize > 0 {
		parts = append(parts, fmt.Sprintf("crew size: %d", after.CrewSize))
	}
	if d := newItems(before.Documents, after.Documents); len(d) > 0 {
		parts = append(parts, "documents: "+strings.Join(d, ", "))
	}
	if before.Availability != after.Availability && strings.TrimSpace(after.Availability) != "" {
		parts = append(parts, "availability: "+after.Availability)
	}
	return strings.Join(parts, "; ")
}

// ApplyEdit applies one small, explicit edit from the profile panel: remove
// a value, or set one. It is the deterministic counterpart to the two LLM
// extraction modes — a visitor clicking a chip never has to wait on the
// model.
func ApplyEdit(p model.Profile, field, op, value string) model.Profile {
	value = strings.TrimSpace(value)
	switch field {
	case "role":
		if op == "remove" {
			p.Role = "unknown"
		} else if value != "" {
			p.Role = strings.ToLower(value)
		}
	case "trades":
		if op == "remove" {
			p.Trades = removeValue(p.Trades, value)
		} else if value != "" {
			p.Trades = mergeList(p.Trades, []string{value})
		}
	case "regions":
		switch {
		case op == "remove":
			p.Regions = removeValue(p.Regions, value)
		case op == "set" && value != "":
			p.Regions = []string{value}
		}
	case "documents":
		if op == "remove" {
			p.Documents = removeValue(p.Documents, value)
		} else if value != "" {
			p.Documents = mergeList(p.Documents, []string{value})
		}
	case "languages":
		if op == "remove" {
			p.Languages = removeValue(p.Languages, value)
		} else if value != "" {
			p.Languages = mergeList(p.Languages, []string{value})
		}
	case "crewSize":
		if op == "remove" {
			p.CrewSize = 0
		} else if n, err := strconv.Atoi(value); err == nil && n > 0 {
			p.CrewSize = n
		}
	case "availability":
		if op == "remove" {
			p.Availability = ""
		} else {
			p.Availability = value
		}
	case "company":
		if op == "remove" {
			p.Company = ""
		} else {
			p.Company = value
		}
	}
	return p
}

// --- turn bookkeeping --------------------------------------------------------

// noteTurn records whether a turn taught the profile anything new, and
// reports whether onboarding should now offer to finish early — after two
// consecutive turns in a row that added nothing, asking a third question is
// worse than stopping.
func (h *Handler) noteTurn(profileID string, learned bool) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if learned {
		delete(h.skips, profileID)
		return false
	}
	h.skips[profileID]++
	return h.skips[profileID] >= 2
}

func (h *Handler) resetTurns(profileID string) {
	h.mu.Lock()
	delete(h.skips, profileID)
	h.mu.Unlock()
}

// --- handlers ---------------------------------------------------------------

type profileView struct {
	model.Profile
	Missing []string
}

type threadView struct {
	Profile     model.Profile
	Question    string
	Done        bool
	Echo        string
	Progress    int
	Learned     bool
	OfferFinish bool
	ProfileView profileView
	StreamURL   string
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	p, err := Ensure(w, r, h.deps)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	question, done := NextQuestion(p)
	h.render(w, threadView{Profile: p, Question: question, Done: done, Progress: store.Completeness(p)})
}

// Turn takes one message, learns from it, and asks the next open question.
// It is exported so the /ask router can send a first-time visitor here. The
// extraction and the resulting question are computed synchronously, so the
// thread always works without JavaScript; when something new was learned, it
// also wires up a streamed one-line acknowledgement as a progressive
// enhancement on top.
func (h *Handler) Turn(w http.ResponseWriter, r *http.Request) {
	message := strings.TrimSpace(firstNonEmpty(r.FormValue("message"), r.URL.Query().Get("message")))
	p, err := Ensure(w, r, h.deps)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if message == "" {
		question, done := NextQuestion(p)
		h.render(w, threadView{Profile: p, Question: question, Done: done, Progress: store.Completeness(p)})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	before := p
	if isCorrection(message) {
		p = ExtractCorrection(ctx, h.deps, p, message)
	} else {
		p = Extract(ctx, h.deps, p, message)
	}
	saved, err := h.deps.Store.UpsertProfile(ctx, p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	learned := !factsEqual(before, saved)
	offerFinish := h.noteTurn(saved.ID, learned)
	question, done := NextQuestion(saved)

	view := threadView{
		Profile:     saved,
		Question:    question,
		Done:        done,
		Echo:        message,
		Progress:    saved.Completeness,
		Learned:     learned,
		OfferFinish: offerFinish && !done,
	}
	if learned {
		view.ProfileView = profileView{Profile: saved, Missing: missingFields(saved)}
		view.StreamURL = "/start/stream?" + url.Values{"hint": {describeLearned(before, saved)}}.Encode()
	}
	h.render(w, view)
}

// stream answers a hint already computed by Turn as server-sent events: one
// "message" event per chunk of a short natural-language acknowledgement,
// then a "done" event that closes the connection. It persists nothing —
// the profile itself was already saved synchronously before the stream
// started, so a dropped connection here can never leave it half-written.
func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	hint := strings.TrimSpace(r.URL.Query().Get("hint"))

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	defer func() {
		writeSSE(w, "done", "")
		flusher.Flush()
	}()

	if hint == "" || ctx.Err() != nil {
		return
	}
	fallback := "Got it — " + hint + "."
	if h.deps.LLM == nil {
		writeSSE(w, "message", html.EscapeString(fallback))
		flusher.Flush()
		return
	}

	req := llm.Request{
		MaxTokens:   256,
		Temperature: 0.4,
		Messages: []llm.Message{
			{Role: "system", Content: "You are the onboarding voice of a construction crew marketplace. " +
				"In one short, warm sentence (under 25 words) confirm exactly the facts listed below — nothing else, " +
				"no greeting, no question, no restating things not listed."},
			{Role: "user", Content: hint},
		},
	}
	var answer strings.Builder
	streamErr := h.deps.LLM.Stream(ctx, req, func(d llm.Delta) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.Content != "" {
			answer.WriteString(d.Content)
			writeSSE(w, "message", html.EscapeString(d.Content))
			flusher.Flush()
		}
		return nil
	})
	if answer.Len() == 0 && ctx.Err() == nil {
		_ = streamErr
		writeSSE(w, "message", html.EscapeString(fallback))
		flusher.Flush()
	}
}

func writeSSE(w http.ResponseWriter, event, data string) {
	fmt.Fprintf(w, "event: %s\n", event)
	if data == "" {
		fmt.Fprint(w, "data: \n")
	} else {
		for _, line := range strings.Split(data, "\n") {
			fmt.Fprintf(w, "data: %s\n", line)
		}
	}
	fmt.Fprint(w, "\n")
}

func (h *Handler) profilePanel(w http.ResponseWriter, r *http.Request) {
	p, ok := Current(r, h.deps)
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	h.renderProfile(w, p)
}

// profileEdit applies one chip-level edit (remove a trade, change a region,
// set crew size, …) and re-renders the panel. It never touches the model —
// these are facts the visitor stated explicitly by clicking or typing.
func (h *Handler) profileEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p, ok := Current(r, h.deps)
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	field := r.FormValue("field")
	op := r.FormValue("op")
	value := r.FormValue("value")
	p = ApplyEdit(p, field, op, value)
	saved, err := h.deps.Store.UpsertProfile(r.Context(), p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.resetTurns(saved.ID)
	h.renderProfile(w, saved)
}

func (h *Handler) renderProfile(w http.ResponseWriter, p model.Profile) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tpl.ExecuteTemplate(w, "profile", profileView{Profile: p, Missing: missingFields(p)}); err != nil {
		log.Printf("onboarding: render profile: %v", err)
	}
}

// finish lets a visitor stop onboarding early — most useful after the
// "offer to finish" nudge — without inventing facts to force completeness.
func (h *Handler) finish(w http.ResponseWriter, r *http.Request) {
	p, ok := Current(r, h.deps)
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	h.resetTurns(p.ID)
	h.render(w, threadView{Profile: p, Done: true, Progress: p.Completeness})
}

func (h *Handler) reset(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) render(w http.ResponseWriter, v threadView) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if v.Echo != "" {
		fmt.Fprintf(w, `<div class="mm-msg you"><span class="mm-who">you</span><p>%s</p></div>`, html.EscapeString(v.Echo))
	}
	if err := h.tpl.ExecuteTemplate(w, "thread", v); err != nil {
		log.Printf("onboarding: render thread: %v", err)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
