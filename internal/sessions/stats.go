package sessions

import (
	"cmp"
	"math"
	"slices"
	"sort"
	"strings"
	"time"
)

// Stats is what every session spent, day by day: all of them, not only the
// latest List reads.
type Stats struct {
	From string `json:"from"` // the first date, local: the range's, or for all time the first with anything
	To   string `json:"to"`   // today
	Days []Day  `json:"days"` // the dates with anything, in order
	// Sessions are the sessions at work in the range, each with what it
	// spent in it, the most recently active first. Too many to send the
	// page over a long range: Overview sums them up.
	Sessions []Summary `json:"-"`
	// Keys are those sessions' keys, agent:id, in the same order: which of
	// the sessions the page lists were at work in the range. The list is
	// every session ever, so without them it guessed from each one's last
	// line — and a session whose last line is a setting or a fork's seed,
	// written long after its last call, was listed but not counted.
	Keys []string `json:"keys"`
	// gateway are the sessions magpie knows only from the gateway's
	// records (MergeExternalStats), the most recently active first. They
	// are kept apart from Sessions, which the merge copied whole to add
	// them: ~10 MB over a full history beside the Sessions it was copied
	// from, kept for each range (#2278).
	gateway []Summary
	// shift is the days added to a session's days for the range's: the
	// gateway's history can start before Sessions' From.
	shift int
}

// eachSession calls f with Sessions and the gateway's, the most recently
// active first, a Sessions one first of two as recent.
func (st *Stats) eachSession(f func(*Summary)) {
	i, j := 0, 0
	for i < len(st.Sessions) || j < len(st.gateway) {
		if j == len(st.gateway) || i < len(st.Sessions) && !st.gateway[j].Last.After(st.Sessions[i].Last) {
			f(&st.Sessions[i])
			i++
		} else {
			f(&st.gateway[j])
			j++
		}
	}
}

// AllSessions is every session st counts, the gateway's among them, in
// eachSession's order, each with its days in the range's: a copy, for a
// reader that wants them as one list.
func (st Stats) AllSessions() []Summary {
	out := make([]Summary, 0, len(st.Sessions)+len(st.gateway))
	st.eachSession(func(s *Summary) { out = append(out, st.shifted(*s)) })
	return out
}

// shifted is s with its days in the range's.
func (st Stats) shifted(s Summary) Summary {
	if st.shift == 0 {
		return s
	}
	s.Days = slices.Clone(s.Days)
	for j := range s.Days {
		s.Days[j] += st.shift
	}
	s.perDay = slices.Clone(s.perDay)
	for j := range s.perDay {
		s.perDay[j].day += st.shift
	}
	return s
}

// Summary is one session's share of a range.
type Summary struct {
	Key           string        `json:"key"` // agent:id as its files are grouped, for Get
	Agent         string        `json:"agent"`
	ID            string        `json:"id"`
	Cwd           string        `json:"cwd"`
	Title         string        `json:"title"`
	Nickname      string        `json:"nickname,omitempty"`
	Last          time.Time     `json:"last"`
	Role          string        `json:"role,omitempty"`
	ParentSession string        `json:"parent_session,omitempty"`
	RootSession   string        `json:"root_session,omitempty"`
	ModelEffort   []ModelEffort `json:"model_effort,omitempty"`
	// Gateway: magpie knows the session only from the gateway's records
	// (MergeExternalStats). The agent left no file magpie reads, so it has
	// no folder, and no prompt unless gateway conversations were recorded
	// (#2285: a Pi subagent's DeepSeek turns read as "(no prompt)").
	Gateway bool `json:"gateway,omitempty"`
	Tokens
	Cost   float64  `json:"cost"`
	Priced bool     `json:"priced"` // every model it used has a price
	Active int64    `json:"active"` // seconds
	Models []string `json:"models"` // the models it used in the range, the most used first
	Days   []int    `json:"days"`   // the dates it was at work on, as days after From
	// Prompts and Replies are the messages each way, ToolCalls the tools
	// it called, in the range (Claude Code's, Codex's, Pi's and omp's sessions)
	Prompts   int `json:"prompts"`
	Replies   int `json:"replies"`
	ToolCalls int `json:"tool_calls"`

	// slices and not maps: each range statsFor keeps holds them for every
	// session, and a map of one or two names is a few hundred bytes
	tools  counts       // calls by tool
	skills counts       // calls by skill, with the last date of each
	perDay []summaryDay // by day after From, in order
}

// summaryDay is a session's day, for the overview's calendar and trends.
type summaryDay struct {
	day              int // after From
	messages, output int
	tools            counts // calls by tool category
}

// counts are calls by name, in the names' order.
type counts []nameCount

type nameCount struct {
	name string
	n    int
	last string // a skill's: the last date it was called
}

// of is name's calls.
func (c counts) of(name string) int {
	if i, ok := slices.BinarySearchFunc(c, name, func(c nameCount, name string) int { return strings.Compare(c.name, name) }); ok {
		return c[i].n
	}
	return 0
}

// Day is one local date's usage, cut fine enough for the page to filter
// it by agent, model and folder itself.
type Day struct {
	Date   string   `json:"date"`
	Usage  []Usage  `json:"usage"`  // by agent, folder and model
	Active []Active `json:"active"` // by agent and folder
}

// Usage is what one model spent for one agent in one folder on a day.
type Usage struct {
	Agent string `json:"agent"`
	Cwd   string `json:"cwd"`
	Model string `json:"model"`
	Tokens
	Cost   float64 `json:"cost"` // USD at the effective price, when priced
	Priced bool    `json:"priced"`
}

// Active is the time an agent's sessions in one folder were at work on a
// day, in seconds (see day.Active for how it is told).
type Active struct {
	Agent   string `json:"agent"`
	Cwd     string `json:"cwd"`
	Seconds int64  `json:"seconds"`
	// Hours is Seconds again by the local hour of the day (24, or none)
	Hours []int64 `json:"hours,omitempty"`
}

// ToolUse is what the sessions called their tools for.
type ToolUse struct {
	Calls      int        `json:"calls"`
	Sessions   int        `json:"sessions"`
	Top        []ToolStat `json:"top"`        // the most called
	Categories []ToolStat `json:"categories"` // by category, the most called first
	// Weeks are the calls week by week, from Monday, by category
	Weeks []ToolWeek `json:"weeks"`
}

// ToolStat is one tool's (or category's) calls.
type ToolStat struct {
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
	Calls    int    `json:"calls"`
	Sessions int    `json:"sessions"`
}

// ToolWeek is one week's calls by category.
type ToolWeek struct {
	Start string         `json:"start"` // its Monday
	Calls map[string]int `json:"calls"`
}

// SkillUse is the skills the sessions called up.
type SkillUse struct {
	Calls int     `json:"calls"`
	Top   []Skill `json:"top"`
	Count int     `json:"count"` // skills called at all
}

// Skill is one skill's calls.
type Skill struct {
	Name     string         `json:"name"`
	Calls    int            `json:"calls"`
	Sessions int            `json:"sessions"`
	Last     string         `json:"last"`     // the date it was last called
	Agents   map[string]int `json:"agents"`   // calls by agent
	Projects []ToolStat     `json:"projects"` // the folders that called it most, by name
}

// Overview sums up st's sessions of agent, with model and in folder cwd
// ("" for any), with the n that spent the most each way.
func (st Stats) Overview(agent, model, cwd string, n int) Overview {
	out := Overview{Days: []int{}, Top: map[string][]Summary{}}
	first, err1 := time.ParseInLocation(time.DateOnly, st.From, time.Local)
	last, err2 := time.ParseInLocation(time.DateOnly, st.To, time.Local)
	if err1 == nil && err2 == nil && !last.Before(first) {
		out.Days = make([]int, int(math.Round(last.Sub(first).Hours()/24))+1)
	}
	out.Messages, out.Output = make([]int, len(out.Days)), make([]int, len(out.Days))
	// the sessions are pointed to, not copied: a Summary is ~400 bytes, and
	// the Sessions page's poll copied all of them four times, ~1.5 GB in
	// two minutes over 34k sessions while an agent worked (#2278)
	all := len(st.Sessions) + len(st.gateway)
	in := make([]*Summary, 0, all)
	spent := make([]int, 0, all)
	st.eachSession(func(s *Summary) {
		if agent != "" && s.Agent != agent || cwd != "" && s.Cwd != cwd || model != "" && !slices.Contains(s.Models, model) {
			return
		}
		in = append(in, s)
		spent = append(spent, s.Input+s.Output)
		for _, d := range s.Days {
			if d += st.shift; d >= 0 && d < len(out.Days) {
				out.Days[d]++
			}
		}
		for _, pd := range s.perDay {
			if d := pd.day + st.shift; d >= 0 && d < len(out.Days) {
				out.Messages[d] += pd.messages
				out.Output[d] += pd.output
			}
		}
	})
	out.Shape = shapes(in)
	// a session's days are after the day st.shift days after From
	out.Tools = toolUse(in, first.AddDate(0, 0, st.shift), n)
	out.Skills = skillUse(in, n)
	out.Count = len(in)
	if len(spent) > 0 {
		sort.Ints(spent)
		// the nearest rank
		rank := func(p float64) int { return spent[max(0, int(math.Ceil(p*float64(len(spent))))-1)] }
		out.Median, out.P90 = rank(0.5), rank(0.9)
	}
	top := make([]*Summary, len(in))
	for by, v := range map[string]func(*Summary) float64{
		"tokens": func(s *Summary) float64 { return float64(s.Input + s.Output) },
		"cost":   func(s *Summary) float64 { return s.Cost },
		"active": func(s *Summary) float64 { return float64(s.Active) },
	} {
		copy(top, in)
		sort.SliceStable(top, func(i, j int) bool { return v(top[i]) > v(top[j]) })
		k := 0
		for k < len(top) && k < n && v(top[k]) > 0 {
			k++
		}
		out.Top[by] = make([]Summary, k)
		for i, s := range top[:k] {
			out.Top[by][i] = st.shifted(*s)
		}
	}
	return out
}

// shapes buckets the sessions that told their messages, or for minutes
// those that were at work.
func shapes(in []*Summary) map[string]Shape {
	out := map[string]Shape{}
	for by, edges := range shapeEdges {
		sh := Shape{Edges: edges, Counts: make([]int, len(edges))}
		for _, s := range in {
			v := s.Prompts + s.Replies
			switch by {
			case "minutes":
				// every agent's sessions tell their time, not all their
				// messages
				if s.Active <= 0 {
					continue
				}
				v = int(max(1, (s.Active+30)/60))
			case "messages", "autonomy":
				if v == 0 {
					continue
				}
			}
			if by == "autonomy" {
				v = s.ToolCalls / max(1, s.Prompts)
			}
			i := len(edges) - 1
			for i > 0 && v < edges[i] {
				i--
			}
			sh.Counts[i]++
			sh.Total++
		}
		out[by] = sh
	}
	return out
}

// toolUse sums up the sessions' tool calls, the n most called. The callers
// spell a tool's name either way ("bash", "Bash"), so its spellings are one
// tool here, shown as its summary spells it.
func toolUse(in []*Summary, first time.Time, n int) ToolUse {
	out := ToolUse{Top: []ToolStat{}, Categories: []ToolStat{}, Weeks: []ToolWeek{}}
	tools, cats := map[string]*ToolStat{}, map[string]*ToolStat{}
	shown, said := map[string]string{}, map[string]int{} // the spelling a tool is shown by, the calls each spelling has
	weeks := map[string]map[string]int{}
	for _, s := range in {
		if s.ToolCalls == 0 {
			continue
		}
		out.Sessions++
		out.Calls += s.ToolCalls
		seen, seenTool := map[string]bool{}, map[string]bool{}
		for _, tc := range s.tools {
			name, c := tc.name, tc.n
			key := strings.ToLower(name) // one tool apiece, its spellings together
			t := tools[key]
			if t == nil {
				t = &ToolStat{Name: name}
				tools[key] = t
			}
			t.Calls += c
			if !seenTool[key] {
				seenTool[key] = true
				t.Sessions++
			}
			said[name] += c
			if best := shown[key]; best == "" || said[name] > said[best] || (said[name] == said[best] && name < best) {
				shown[key] = name // its most called spelling, the first in byte order on a tie
			}
			// the legend counts each spelling under its own category, as
			// the weeks do (pd.tools): "task" and "Task" are two there
			k := ToolCategory(name)
			cat := cats[k]
			if cat == nil {
				cat = &ToolStat{Name: k}
				cats[k] = cat
			}
			cat.Calls += c
			if !seen[k] {
				seen[k] = true
				cat.Sessions++
			}
		}
		if first.IsZero() {
			continue
		}
		for _, pd := range s.perDay {
			day := first.AddDate(0, 0, pd.day)
			mon := day.AddDate(0, 0, -(int(day.Weekday())+6)%7).Format(time.DateOnly)
			w := weeks[mon]
			if w == nil {
				w = map[string]int{}
				weeks[mon] = w
			}
			for _, c := range pd.tools {
				w[c.name] += c.n
			}
		}
	}
	for key, t := range tools {
		t.Name = shown[key]
		t.Category = ToolCategory(t.Name)
		if strings.EqualFold(t.Name, t.Category) {
			t.Name = t.Category // "bash" shown as the summary's "Bash"
		}
		out.Top = append(out.Top, *t)
	}
	for _, c := range cats {
		out.Categories = append(out.Categories, *c)
	}
	byCalls := func(a, b ToolStat) int { return cmp.Or(cmp.Compare(b.Calls, a.Calls), cmp.Compare(a.Name, b.Name)) }
	slices.SortFunc(out.Top, byCalls)
	slices.SortFunc(out.Categories, byCalls)
	out.Top = out.Top[:min(len(out.Top), n)]
	for mon, calls := range weeks {
		out.Weeks = append(out.Weeks, ToolWeek{Start: mon, Calls: calls})
	}
	slices.SortFunc(out.Weeks, func(a, b ToolWeek) int { return cmp.Compare(a.Start, b.Start) })
	return out
}

// skillUse sums up the skills the sessions called up, the n most called.
func skillUse(in []*Summary, n int) SkillUse {
	out := SkillUse{Top: []Skill{}}
	skills := map[string]*Skill{}
	projects := map[string]map[string]int{}
	for _, s := range in {
		for _, sc := range s.skills {
			name, c := sc.name, sc.n
			k := skills[name]
			if k == nil {
				k = &Skill{Name: name, Agents: map[string]int{}, Projects: []ToolStat{}}
				skills[name] = k
				projects[name] = map[string]int{}
			}
			k.Calls += c
			k.Sessions++
			k.Agents[s.Agent] += c
			k.Last = max(k.Last, sc.last)
			projects[name][s.Cwd] += c
			out.Calls += c
		}
	}
	out.Count = len(skills)
	for name, k := range skills {
		for cwd, c := range projects[name] {
			k.Projects = append(k.Projects, ToolStat{Name: cwd, Calls: c})
		}
		slices.SortFunc(k.Projects, func(a, b ToolStat) int { return cmp.Or(cmp.Compare(b.Calls, a.Calls), cmp.Compare(a.Name, b.Name)) })
		k.Projects = k.Projects[:min(len(k.Projects), 3)]
		out.Top = append(out.Top, *k)
	}
	slices.SortFunc(out.Top, func(a, b Skill) int { return cmp.Or(cmp.Compare(b.Calls, a.Calls), cmp.Compare(a.Name, b.Name)) })
	out.Top = out.Top[:min(len(out.Top), n)]
	return out
}

// ToolCategory is the kind of tool a name is, Claude Code's and Codex's
// alike: Bash, Edit, Read, Write, Glob, Grep, Task (subagents and plans),
// Tool (an MCP server's) or Other.
func ToolCategory(name string) string {
	switch name {
	case "Bash", "BashOutput", "KillShell", "KillBash", "exec_command", "shell", "shell_command", "local_shell_call", "write_stdin", "unified_exec", "bash", "powershell":
		return "Bash"
	case "Edit", "MultiEdit", "NotebookEdit", "apply_patch", "edit", "str_replace":
		return "Edit"
	case "Read", "NotebookRead", "view_image", "read_file", "view", "read":
		return "Read"
	case "Write", "write_file", "create", "write":
		return "Write"
	case "Glob", "LS", "list_dir", "glob", "find", "ls", "fffind":
		return "Glob"
	case "Grep", "grep", "grep_files", "search", "ffgrep":
		return "Grep"
	case "Task", "Agent", "TodoWrite", "update_plan", "spawn_agent", "send_input", "wait", "close_agent", "Workflow":
		return "Task"
	}
	if strings.HasPrefix(name, "mcp__") || strings.HasPrefix(name, "mcp_") {
		return "Tool"
	}
	return "Other"
}

// Overview is what the sessions of a range come to under the page's
// filters.
type Overview struct {
	Count  int   `json:"count"`
	Median int   `json:"median"` // tokens (in and out) of the middle session
	P90    int   `json:"p90"`    // … of the one nine in ten spent no more than
	Days   []int `json:"days"`   // sessions at work each day, from Stats.From to To
	// Top are the sessions that spent the most, by tokens, cost and
	// active time, n of each
	Top map[string][]Summary `json:"top"`
	// Messages and Output are the messages and output tokens each day,
	// as Days
	Messages []int `json:"messages"`
	Output   []int `json:"output"`
	// Shape is how many sessions came to how much: by messages, by
	// minutes at work and by tool calls a prompt (autonomy)
	Shape  map[string]Shape `json:"shape"`
	Tools  ToolUse          `json:"tools"`
	Skills SkillUse         `json:"skills"`
}

// Shape counts sessions into buckets: Counts[i] those from Edges[i] up to
// the next edge, the last one open.
type Shape struct {
	Edges  []int `json:"edges"`
	Counts []int `json:"counts"`
	Total  int   `json:"total"`
}

// shapeEdges are the buckets' lower edges for each way of telling.
var shapeEdges = map[string][]int{
	"messages": {1, 6, 16, 31, 61, 121},
	"minutes":  {1, 6, 16, 31, 61, 121},
	"autonomy": {0, 1, 3, 6, 11, 21},
}
