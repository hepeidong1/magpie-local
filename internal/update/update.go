// Package update keeps magpie current. Builds are published as GitHub
// releases of yetone/magpie-releases; usemagpie.ai/api/latest describes the
// newest one: its version, notes, and every file with its SHA-256.
//
// The macOS app replaces its own bundle: the new zip is downloaded, checked
// against its hash, unpacked, and accepted only if it is signed by the same
// team as the app already installed. A bare binary (the terminal build,
// and the desktop app on Windows and Linux) replaces itself the same way,
// minus the signature.
package update

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/yetone/magpie/internal/netproxy"
	"github.com/yetone/magpie/internal/source"
)

// Site is magpie's home; its /api/latest is the update feed.
const Site = "https://usemagpie.ai"

// Feed is where the newest release is described. MAGPIE_UPDATE_FEED points
// it elsewhere, for testing an update against a local server.
func Feed() string {
	if f := os.Getenv("MAGPIE_UPDATE_FEED"); f != "" {
		return f
	}
	return Site + "/api/latest"
}

// Release is one published version.
type Release struct {
	Version string           `json:"version"` // "0.2.0", no v
	Notes   string           `json:"notes"`   // markdown
	URL     string           `json:"url"`     // the release page
	Assets  map[string]Asset `json:"assets"`  // by file name
}

// Asset is one downloadable file of a release.
type Asset struct {
	URL    string `json:"url"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// client asks the feed and downloads a release through the proxy the rest
// of magpie's requests take — Settings' Proxy, else the environment's, else
// the system's (#294) — on a transport of its own, whatever the process has
// done to http.DefaultTransport.
var client = &http.Client{Timeout: 10 * time.Minute, Transport: proxied()}

func proxied() http.RoundTripper {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = netproxy.Func
	return netproxy.Dispatch(t)
}

// Latest asks the feed for the newest release, its notes in English.
func Latest(ctx context.Context) (*Release, error) { return LatestIn(ctx, "") }

// LatestIn asks the feed for the newest release, its notes in lang (see
// InLang): the app's language, which What's new follows. On Settings'
// Stable channel it is Stable's release (see Stable).
func LatestIn(ctx context.Context, lang string) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", withLang(Feed(), lang), nil)
	if err != nil {
		return nil, err
	}
	res, err := source.Do(client, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("update feed: %s", res.Status)
	}
	var r Release
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("update feed: %w", err)
	}
	if parse(r.Version) == nil {
		return nil, fmt.Errorf("update feed: no version")
	}
	r.Notes = InLang(r.Notes, lang)
	return &r, nil
}

// Released reports whether v is a release version rather than a build from
// source ("dev", "0bcb2cc-dirty", git describe's "v0.1.0-3-g0bcb2cc");
// only releases update themselves.
func Released(v string) bool {
	s := parse(v)
	return s != nil && !describe.MatchString(s.pre) && !strings.Contains(s.pre, "dirty")
}

// describe matches what git describe adds after a tag: commits since, hash.
var describe = regexp.MustCompile(`^\d+-g[0-9a-f]+`)

// Newer reports whether version a comes after b. A pre-release comes before
// the release it leads up to.
func Newer(a, b string) bool {
	x, y := parse(a), parse(b)
	if x == nil || y == nil {
		return false
	}
	for i := range 3 {
		if x.n[i] != y.n[i] {
			return x.n[i] > y.n[i]
		}
	}
	return comparePre(x.pre, y.pre) > 0
}

// comparePre orders two pre-releases the way semver does (§11): no
// pre-release comes after any; otherwise identifier by identifier, numbers
// as numbers ("beta.10" after "beta.9"), a number before a word, and a
// shorter list before a longer one it starts.
func comparePre(a, b string) int {
	switch {
	case a == b:
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	}
	x, y := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(x) && i < len(y); i++ {
		m, errM := strconv.ParseUint(x[i], 10, 64)
		n, errN := strconv.ParseUint(y[i], 10, 64)
		switch {
		case errM == nil && errN == nil:
			if m != n {
				return cmp.Compare(m, n)
			}
		case errM == nil:
			return -1
		case errN == nil:
			return 1
		default:
			if c := strings.Compare(x[i], y[i]); c != 0 {
				return c
			}
		}
	}
	return cmp.Compare(len(x), len(y))
}

type semver struct {
	n   [3]int
	pre string
}

func parse(v string) *semver {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	// build metadata ("+sha.5d0c1dc") plays no part in the order
	v, _, _ = strings.Cut(v, "+")
	v, pre, _ := strings.Cut(v, "-")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return nil
	}
	var s semver
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil
		}
		s.n[i] = n
	}
	s.pre = pre
	return &s
}

// GUI says this binary has the desktop app in it. Off the Mac that app is a
// single binary too, and it updates from its own build, not the terminal
// one.
func Bundle() string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	exe, err := Executable()
	if err != nil {
		return ""
	}
	// …/magpie.app/Contents/MacOS/magpie
	app := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
	if filepath.Ext(app) != ".app" || filepath.Base(filepath.Dir(exe)) != "MacOS" {
		return ""
	}
	return app
}

// Homebrew is whether exe was installed by Homebrew (brew install magpie),
// which keeps it in its Cellar and has to be the one to upgrade it: a
// binary replaced under it leaves brew thinking the old version is there.
func Executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// Writable reports whether magpie may replace what lives in dir.
func Alive(pid int) bool { return alive(pid) }

// OpenBeside opens bundle now, a new instance beside this one (open -n),
// tried again while open fails: a restart that hands the gateway over
// (internal/handoff) starts the new version before this one quits. With
// window, its window; else its menu bar icon only, as Relaunch, in the
// background as Relaunch's when quiet or with no window.

// Writable reports whether magpie may replace what lives in dir.
func Writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".magpie-update-*")
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(f.Name())
	return true
}
