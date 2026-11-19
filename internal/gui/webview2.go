package gui

import (
	"fmt"
	"slices"
	"strings"
)

// magpie's windows are WebView2 on Windows, a runtime Windows 10 doesn't
// always have. Without it Wails hands the failure to ErrorHandler and exits,
// and the desktop build has no console for that to land in: a double-click
// came up to nothing (#1806, ufec). Run asks before any window is made and,
// missing, says so in a box of Windows' own.

// webView2Page is Microsoft's download page for the runtime.
const webView2Page = "https://developer.microsoft.com/microsoft-edge/webview2/"

// webView2Missing reads Wails' own answer, Env.Info().PlatformInfo's
// "WebView2": the runtime's version, "" when its lookup failed, the lookup
// a window's WebView2 environment makes. A key Wails doesn't set at all
// says nothing, and is read as there: refusing to start a magpie that would
// have worked is worse than the silent exit.
func webView2Missing(info map[string]any) bool {
	v, ok := info["WebView2"]
	return ok && fmt.Sprint(v) == ""
}

// webView2Notice is the box's title and text in lang (trayLang's), English
// where it has none. It names the page and winget's package (#1806: ufec
// asked for the winget way too), and asks whether to open the page; Yes
// opens it.
func webView2Notice(_ string) (title, body string) {
	t := webView2Words
	return t[0], fmt.Sprintf(t[1], webView2Page)
}

// webView2Words is the notice in this build's one language, Chinese.
var webView2Words = [2]string{"magpie 需要 WebView2",
	"magpie 的窗口需要 Microsoft Edge WebView2 运行时，这台电脑上没有安装。\r\n\r\n" +
		"请从这里安装：\r\n%s\r\n\r\n或在终端里运行：\r\nwinget install Microsoft.EdgeWebView2Runtime\r\n\r\n" +
		"装好之前，可以用 \"magpie web\" 在浏览器里打开 magpie，或用 \"magpie tui\" 在终端里打开。\r\n\r\n" +
		"现在打开下载页面吗？"}

// webView2Args sorts what the user put in WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS
// (webview2env.Args) into Wails' three options, which it passes as the
// app's own arguments: the features to disable, the ones to enable, and
// every other argument as written. Wails' loader blanks the variable
// itself, so without this a user's --disable-features=RendererCodeIntegrity
// never reached magpie's renderer (#2753, LuYquan: an injected DLL got it
// killed and the window went black). Wails puts its own
// msSmartScreenProtection first in DisabledFeatures and writes the list as
// one --disable-features (and EnabledFeatures as one --enable-features):
// Chromium keeps only the last of a repeated switch, so two would drop one
// of them. A list given more than once is merged, each feature once, in
// the order given. Arguments are split as Windows splits a command line on
// spaces, a double-quoted run kept whole.
func webView2Args(env string) (disabled, enabled, other []string) {
	for _, a := range splitArgs(env) {
		switch k, v, _ := strings.Cut(a, "="); k {
		case "--disable-features":
			disabled = addFeatures(disabled, v)
		case "--enable-features":
			enabled = addFeatures(enabled, v)
		default:
			other = append(other, a)
		}
	}
	return disabled, enabled, other
}

// addFeatures adds a comma list's features to have, each once.
func addFeatures(have []string, list string) []string {
	for _, f := range strings.Split(strings.Trim(list, `"`), ",") {
		if f = strings.TrimSpace(f); f != "" && !slices.Contains(have, f) {
			have = append(have, f)
		}
	}
	return have
}

// splitArgs splits a command line on spaces and tabs outside double quotes,
// keeping each argument as written, its quotes too.
func splitArgs(s string) []string {
	var out []string
	var cur strings.Builder
	quoted := false
	for _, r := range s {
		switch {
		case r == '"':
			quoted = !quoted
			cur.WriteRune(r)
		case (r == ' ' || r == '\t' || r == '\n' || r == '\r') && !quoted:
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}
