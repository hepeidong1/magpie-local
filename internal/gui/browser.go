package gui

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/yetone/magpie/internal/access"
	"github.com/yetone/magpie/internal/browserext"
	"github.com/yetone/magpie/internal/gateway"
	"github.com/yetone/magpie/internal/settings"
)

// Magpie Browser's extension, as the Library page sets it up: written to a
// folder the user can see, in their home folder, which they drop on their
// own browser's chrome://extensions (or choose with Load unpacked), and
// connected or not as the gateway says.

// browserExtDir is where the extension is written: in the home folder,
// where Finder and Explorer show it, to drag from or choose.
func browserExtDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(settings.Dir(), "browser-extension")
	}
	return filepath.Join(home, "Magpie Browser Extension")
}

// browserExtOldDir is where v0.1.2442 wrote it, a hidden folder no one
// could find to load; a browser that loaded it from there still does, so
// it is kept up to date too while it is there.
func browserExtOldDir() string { return filepath.Join(settings.Dir(), "browser-extension") }

type browserJSON struct {
	Dir       string `json:"dir"`
	Written   bool   `json:"written"`
	Version   string `json:"version"`
	Connected bool   `json:"connected"`
	Extension string `json:"extension,omitempty"` // the connected one's version
	Browser   string `json:"browser,omitempty"`
	// Drag: the page may drag the folder out of the window itself
	Drag bool `json:"drag,omitempty"`
}

func browserState(ctx context.Context) browserJSON {
	out := browserJSON{Dir: browserExtDir(), Version: browserext.Version(), Drag: canDragFile}
	for _, dir := range []string{out.Dir, browserExtOldDir()} {
		if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err == nil {
			out.Written = out.Written || dir == out.Dir
			// kept as this magpie has it, and pointed at its gateway: the
			// browser reads the folder again when it starts
			browserext.Write(dir, gateway.URL())
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gateway.URL()+gateway.BrowserStatusPath, nil)
	if err != nil {
		return out
	}
	req.Header.Set("Authorization", "Bearer "+access.PrimarySecret())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return out
	}
	defer resp.Body.Close()
	var st struct {
		Connected bool   `json:"connected"`
		Extension string `json:"extension"`
		Browser   string `json:"browser"`
	}
	if resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&st) == nil {
		out.Connected, out.Extension, out.Browser = st.Connected, st.Extension, st.Browser
	}
	return out
}

// revealIn shows path selected in its parent folder where the host can,
// and opens it where it can't.
func revealIn(w Windows, path string) error {
	if r, ok := w.(interface{ RevealFolder(string) error }); ok {
		return r.RevealFolder(path)
	}
	return w.OpenFolder(path)
}
