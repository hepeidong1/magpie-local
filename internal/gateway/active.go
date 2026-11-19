package gateway

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Active is one agent request in flight, as /v1/magpie/concurrency lists
// it for a status bar's hover (#2132, bigbitbox): which agent asked, for
// which model, since when, and where it stands. It holds no prompt, reply
// or key. Model and Provider are the routing trace's, once the request has
// been routed; Phase is said only when the gateway knows it: "slot" while
// it waits in a key's or account's queue (MaxConcurrency), "rpm" while it
// waits for room in the minute (MaxRPM), "upstream" while a try is out at
// the vendor (the request sent, or its reply streaming). A request served
// another way (a subscription bridge, a title helper) has none.
type Active struct {
	ID       int64     `json:"id"`
	Agent    string    `json:"agent"`
	Model    string    `json:"model,omitempty"`
	Provider string    `json:"provider,omitempty"`
	Session  string    `json:"session,omitempty"`
	Started  time.Time `json:"started"`
	Millis   int64     `json:"ms"`
	Phase    string    `json:"phase,omitempty"`
	Route    int64     `json:"route,omitempty"` // its routing trace's id
	// Served: the model the request's current try went to (a routing
	// group's member), and Entry: that entry's own 6-character code —
	// the Requests page's in-flight table reads both; both said once
	// the request has been routed
	Served string `json:"served,omitempty"`
	Entry  string `json:"entry,omitempty"`
	// Slot is the try's slot key ("pid#keyid" / "pid@user" / "pid"): the
	// Requests page resolves the key's name and the entry's concurrency
	// slot number from it
	Slot string `json:"slot,omitempty"`
	// Tries is the request's whole walk so far, one a try: where it went
	// and whether it came back. The routing group's entry board reads it —
	// the tries behind it paint their balls red, the one it waits on now
	// orange — where the last try alone (Served/Entry/Slot) would leave
	// the walk's earlier stops green.
	Tries []ActiveTry `json:"tries,omitempty"`
	// InTokens is how long the request's input was estimated as it was
	// routed, InImages whether it carries an image: the entry board
	// initializes a lane's balls with them — green where the entry would
	// take the request, gray where it wouldn't.
	InTokens int  `json:"inTokens,omitempty"`
	InImages bool `json:"inImages,omitempty"`
}

// ActiveTry is one try of a request in flight (Route.Tries as the board
// reads it): the model it went to, its slot key, the entry's own key
// ("provider/model@cred"), and whether it came back.
type ActiveTry struct {
	Model  string `json:"model,omitempty"`
	Slot   string `json:"slot,omitempty"`
	Entry  string `json:"entry,omitempty"`
	Done   bool   `json:"done,omitempty"`
	Status int    `json:"status,omitempty"`
}

// inflight is a request counted for Busy, kept on its context so routing
// can tell it its route and its phase.
type inflight struct {
	id      int64
	agent   string
	session string
	start   time.Time
	route   atomic.Pointer[Route]
	phase   atomic.Value // string
	// begun: every route begun for it, the first its own, for endBroken
	mu    sync.Mutex
	begun []*Route
	// unrouted: its watch while no route is begun for it (unrouted.go)
	unrouted *unroutedHold
	// tr: the trace its routes are in, for the phase they say (under mu)
	tr *trace
}

type inflightKey struct{}

// inflights are the requests counted for Busy, by id.
type inflights struct {
	mu  sync.Mutex
	seq int64
	m   map[int64]*inflight
}

// add counts r in and answers it with its inflight on its context, and the
// func that counts it out.
func (l *inflights) add(r *http.Request) (*http.Request, func()) {
	f := &inflight{agent: callerFrom(r).agent, session: sessionOf(r.Header), start: time.Now()}
	l.mu.Lock()
	if l.m == nil {
		l.m = map[int64]*inflight{}
	}
	l.seq++
	f.id = l.seq
	l.m[f.id] = f
	l.mu.Unlock()
	return r.WithContext(context.WithValue(r.Context(), inflightKey{}, f)), func() {
		l.mu.Lock()
		delete(l.m, f.id)
		l.mu.Unlock()
	}
}

// inflightOf is ctx's request in flight, nil when it isn't counted.
func inflightOf(ctx context.Context) *inflight {
	f, _ := ctx.Value(inflightKey{}).(*inflight)
	return f
}

// notePhase says where ctx's request in flight stands now, and answers the
// func that puts back where it stood before: a try out at the vendor whose
// own requests wait for room in the minute (rpmTransport) is out at the
// vendor again once they go.
//
// A phase is a wait magpie knows of (noteWait): one before a route is
// begun has unroutedMost, not unroutedUnknown.
func notePhase(ctx context.Context, phase string) (restore func()) {
	f := inflightOf(ctx)
	if f == nil {
		return func() {}
	}
	was, _ := f.phase.Load().(string)
	f.phase.Store(phase)
	waited := noteWait(ctx, phase)
	return func() { waited(); f.phase.Store(was) }
}

// noteRoute ties ctx's request in flight to its routing trace, the first
// route begun for it: the one for what the agent asked.
func noteRoute(ctx context.Context, rt *Route) {
	if f := inflightOf(ctx); f != nil {
		f.route.CompareAndSwap(nil, rt)
		f.mu.Lock()
		f.begun = append(f.begun, rt)
		var shown *Route
		if f.unrouted != nil {
			shown = f.unrouted.routed()
		}
		f.mu.Unlock()
		if shown != nil {
			// the route that showed it while it waited gives way to its own
			f.unrouted.s.trace.drop(shown)
		}
	}
}

// panicEnded is what a route ended by endBroken says.
const panicEnded = "magpie broke off this request: an internal error (in magpie's log)"

// endBroken ends every route begun for f that its handler left in flight
// when it panicked, and each of their tries still out: net/http recovers
// the panic and drops the connection, but a route not done shows as still
// answering on the Routing page and counts as in progress for good, and
// the trace never trims it (#2386).
func (s *Server) endBroken(f *inflight) {
	if f == nil {
		return
	}
	f.mu.Lock()
	begun := append([]*Route(nil), f.begun...)
	f.mu.Unlock()
	for _, rt := range begun {
		s.trace.update(rt, func(r *Route) {
			if r.Done {
				return
			}
			ms := time.Since(r.Time).Milliseconds()
			for i := range r.Tries {
				if t := &r.Tries[i]; !t.Done {
					t.Done, t.Status, t.Error, t.Millis = true, http.StatusInternalServerError, panicEnded, time.Since(t.Start).Milliseconds()
				}
			}
			r.Done, r.Status, r.Error, r.Millis = true, http.StatusInternalServerError, panicEnded, ms
		})
	}
}

// Active lists the agent requests in flight now, the oldest first; empty,
// never nil, when there are none.
func (s *Server) Active() []Active {
	s.active.mu.Lock()
	fs := make([]*inflight, 0, len(s.active.m))
	for _, f := range s.active.m {
		fs = append(fs, f)
	}
	s.active.mu.Unlock()
	now := time.Now()
	out := make([]Active, 0, len(fs))
	for _, f := range fs {
		a := Active{ID: f.id, Agent: f.agent, Session: f.session, Started: f.start, Millis: now.Sub(f.start).Milliseconds()}
		if p, _ := f.phase.Load().(string); p != "" {
			a.Phase = p
		}
		if rt := f.route.Load(); rt != nil {
			s.trace.mu.Lock()
			a.Route, a.Model, a.Provider = rt.ID, rt.Model, rt.Provider
			if rt.Agent != "" {
				a.Agent = rt.Agent
			}
			if a.Session == "" {
				a.Session = rt.Session
			}
			a.InTokens, a.InImages = rt.InTokens, rt.InImages
			// the whole walk so far, the last try first said again in
			// Served/Entry/Slot for the Requests page's board
			for _, t := range rt.Tries {
				a.Tries = append(a.Tries, ActiveTry{Model: t.Model, Slot: t.ID, Entry: activeEntryCode(rt, &t), Done: t.Done, Status: t.Status})
			}
			// the try out at the vendor now: the model it went to, and
			// the entry's own code, for the Requests page's board
			if n := len(rt.Tries); n > 0 {
				t := rt.Tries[n-1]
				a.Served = t.Model
				a.Slot = t.ID
				a.Entry = activeEntryCode(rt, &t)
			}
			s.trace.mu.Unlock()
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
