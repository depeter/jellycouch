package app

import "log/slog"

// AppState represents the top-level application mode.
type AppState int

const (
	StateBrowse AppState = iota
	StatePlay
	StateWeb
)

// String returns a human-readable state name for logs.
func (s AppState) String() string {
	switch s {
	case StateBrowse:
		return "browse"
	case StatePlay:
		return "play"
	case StateWeb:
		return "web"
	default:
		return "unknown"
	}
}

// validTransitions encodes the legal state-transition graph. Any transition
// not in this set is a programming error and is logged (but not rejected —
// we prefer to log and continue over crashing in production).
var validTransitions = map[AppState]map[AppState]bool{
	StateBrowse: {StatePlay: true, StateWeb: true, StateBrowse: true},
	StatePlay:   {StateBrowse: true, StatePlay: true},
	StateWeb:    {StateBrowse: true, StateWeb: true},
}

// setState is the single chokepoint for AppState mutation. It logs every
// transition and warns on any transition not in validTransitions. Callers
// are still responsible for invoking cleanup before a transition — setState
// does not roll back side effects.
func (g *Game) setState(next AppState) {
	prev := g.State
	if prev == next {
		return
	}
	if allowed := validTransitions[prev]; !allowed[next] {
		slog.Warn("unexpected state transition", "from", prev, "to", next)
	} else {
		slog.Info("state transition", "from", prev, "to", next)
	}
	g.State = next
}
