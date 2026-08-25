package store

import (
	"time"

	"mm-machine/internal/model"
)

// SeedOffers is the initial pipeline content, inserted on first boot.
func SeedOffers() []model.Offer {
	now := time.Now()
	return []model.Offer{
		{ID: "MM-1842", Title: "Photovoltaic roof installation", Location: "Munich, DE", Category: "Energy", Amount: "420 panels", Budget: "EUR 146k", Status: "process", Signal: "Attention", Supplier: "Voltwerk GmbH", Progress: 68, Attention: "3 requests need document expiry checks", CreatedAt: now.Add(-72 * time.Hour), UpdatedAt: now.Add(-12 * time.Minute)},
		{ID: "MM-1841", Title: "Retail floor refit", Location: "Zurich, CH", Category: "Interior", Amount: "1,800 m2", Budget: "EUR 82k", Status: "requested", Signal: "OK", Supplier: "Alpine Montage", Progress: 36, Attention: "5 supplier answers ready", CreatedAt: now.Add(-96 * time.Hour), UpdatedAt: now.Add(-38 * time.Minute)},
		{ID: "MM-1838", Title: "Warehouse steel assembly", Location: "Rotterdam, NL", Category: "Industrial", Amount: "96 tons", Budget: "EUR 310k", Status: "open", Signal: "OK", Supplier: "Nordline Build", Progress: 22, Attention: "Hardware list confirmed", CreatedAt: now.Add(-120 * time.Hour), UpdatedAt: now.Add(-time.Hour)},
		{ID: "MM-1832", Title: "Hotel bathroom modernization", Location: "Vienna, AT", Category: "Sanitary", Amount: "74 rooms", Budget: "EUR 228k", Status: "done", Signal: "Review", Supplier: "Prime Install", Progress: 100, Attention: "Review window open", CreatedAt: now.Add(-400 * time.Hour), UpdatedAt: now.Add(-26 * time.Hour)},
	}
}

// SeedPerspectives is the two-sided product story.
func SeedPerspectives() []model.Perspective {
	return []model.Perspective{
		{
			Key:       "owner",
			Label:     "Generalunternehmer",
			Title:     "Mobilize verified teams without broker margin leakage.",
			Subtitle:  "For project owners and GUs who need reliable capacity fast, with proof, clean communication, and payment control.",
			Quote:     "27 Euro paid, 18 Euro reaches the worker. Montage Manager makes the hidden 9 Euro visible and negotiable.",
			Primary:   "Post structured project",
			Secondary: "Compare supplier answers",
			Stats: []model.Metric{
				{Label: "Broker leakage", Value: "30-50%", Note: "margin often captured without operational value"},
				{Label: "Rework driver", Value: "48%", Note: "caused by miscommunication"},
				{Label: "Mobilization window", Value: "14d", Note: "typical pressure for international teams"},
			},
			Workflow:   []string{"Post chaotic brief", "AI normalizes scope", "Invite verified teams", "Compare answers and rates", "Track logbook and approvals", "Release payment and review"},
			Pain:       []string{"Middlemen hide true labor economics", "Site reality differs from the description", "No objective basis for partial acceptance", "Team capacity is hard to verify quickly"},
			ActionName: "Open GU cockpit",
		},
		{
			Key:       "executor",
			Label:     "Subunternehmer",
			Title:     "Win direct work and keep documents ready before the project starts.",
			Subtitle:  "For subcontractors who need direct access to serious projects, predictable payment, team formation, and EU compliance support.",
			Quote:     "A1 can take three weeks for a four-week job. The platform turns document chaos into a reusable trust profile.",
			Primary:   "Find direct projects",
			Secondary: "Prepare compliance safe",
			Stats: []model.Metric{
				{Label: "Late payments", Value: "77%", Note: "subcontractor projects affected"},
				{Label: "Avg. delay", Value: "57d", Note: "cash-flow risk after work is done"},
				{Label: "Penalty risk", Value: "500k", Note: "possible compliance fines in severe cases"},
			},
			Workflow:   []string{"Build trust profile", "Upload A1 and insurance", "Join or form team", "Answer projects directly", "Document progress", "Get accepted and paid"},
			Pain:       []string{"Documents are scattered and expire silently", "GU payment reputation is invisible", "Small teams cannot prove capacity", "Disputes drag because proof is weak"},
			ActionName: "Open SU cockpit",
		},
	}
}

// SeedModules lists the product modules.
func SeedModules() []model.Module {
	return []model.Module{
		{Name: "AI Job Assistant", Body: "Turns photos, voice notes, drawings, and rough text into a normalized project package.", Impact: "Cuts bidder questions and scope conflict."},
		{Name: "Team Builder", Body: "Lets subcontractors combine crews, skills, languages, documents, and hardware into deployable teams.", Impact: "Makes large projects accessible without a broker."},
		{Name: "Document Safe", Body: "Stores A1, insurance, certificates, tax data, expiry dates, and trust-member proof.", Impact: "Speeds EU compliance checks."},
		{Name: "Status Documentation", Body: "Photo, video, timestamp, creator, location, request, update, and logbook history.", Impact: "Creates the payment and acceptance record."},
		{Name: "Dispute Desk", Body: "Structured evidence and neutral review flow for defects, delays, and payment disagreements.", Impact: "Avoids slow court escalation."},
	}
}

// SeedRoadmap lists the phased plan.
func SeedRoadmap() []model.RoadmapItem {
	return []model.RoadmapItem{
		{Phase: "Months 1-3", Title: "MVP: DACH marketplace spine", Body: "Job posting, team formation, basic verification, offer lists, and two role dashboards."},
		{Phase: "Months 4-6", Title: "AI and dispute layer", Body: "AI brief normalization, bidder Q&A, photo history, and first mediation workflow."},
		{Phase: "Months 7-12", Title: "Payments and bureaucracy", Body: "Payment rails, A1/SIPSI/VOB/B modules, enterprise compliance APIs, and premium settlement flows."},
	}
}
