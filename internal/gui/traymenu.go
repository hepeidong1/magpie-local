package gui

import (
	"fmt"
)

// trayZh is the tray menu in Chinese; the page has its own words
// (i18n.js), and these few are all the menu and the icon's tooltip show
// (#301).
var trayZh = map[string]string{
	"Open magpie":                          "打开 magpie",
	"Version %s":                           "版本 %s",
	"Restart to Update":                    "重启以更新",
	"Restart to Update to %s":              "重启以更新到 %s",
	"Quit magpie":                          "退出 magpie",
	"Restart Now to Update":                "立即重启以更新",
	"Restart Now to Update (%d in flight)": "立即重启以更新（%d 个进行中）",
	// the tray icon's tooltip (trayusage.go; yi_time on Discord)
	"%s used":                             "已用 %s",
	"%s left":                             "剩余 %s",
	"pace %s":                             "增速 %s",
	"resets in %s":                        "%s 后重置",
	"%dd %dh":                             "%d 天 %d 小时",
	"%dh %dm":                             "%d 小时 %d 分",
	"%dm":                                 "%d 分钟",
	"as of %s, couldn't be read just now": "截至 %s，暂时无法获取最新额度",
	"Jan 2 15:04":                         "1月2日 15:04",
	"Today in magpie":                     "magpie 今日",
	"Last 7 days in magpie":               "magpie 近 7 天",
	"%d requests":                         "%d 个请求",
	"%d request":                          "%d 个请求",
	", %d with no known price":            "，其中 %d 个没有已知价格",
	"%s in":                               "输入 %s",
	"%s out":                              "输出 %s",
	"%s cached":                           "缓存 %s",
	"%s tokens (%s)":                      "%s token（%s）",
	"Today: no requests yet":              "今天：还没有请求",
	"Today: %s":                           "今天：%s",
}

// trayWords is the tray menu and the icon tooltip: this build speaks
// Chinese alone.
var trayWords = map[string]map[string]string{"zh": trayZh}

// onLang relabels the tray menu when the Settings page changes the
// language; set by the process that has the tray.
var onLang func()

// trayLang is the tray menu's language: this build speaks Chinese alone,
// whatever a saved setting says.
func trayLang(string, func() string) string { return "zh" }

// notesLang is the language release notes are asked in: Chinese.
func notesLang(string) string { return "zh" }

// trayText is a menu line in the language, English where it has none.
func trayText(lang, key string, args ...any) string {
	if s, ok := trayWords[lang][key]; ok {
		key = s
	}
	if len(args) == 0 {
		return key
	}
	return fmt.Sprintf(key, args...)
}

// trayLabels are the tray menu's lines: open, version and quit.
type trayLabels struct{ open, version, quit string }

// trayMenuLabels are the lines in the language.
func trayMenuLabels(lang, version string) trayLabels {
	return trayLabels{
		open:    trayText(lang, "Open magpie"),
		version: trayText(lang, "Version %s", version),
		quit:    trayText(lang, "Quit magpie"),
	}
}
