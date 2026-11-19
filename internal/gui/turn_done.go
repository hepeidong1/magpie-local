package gui

import (
	"fmt"
	"time"

	"github.com/yetone/magpie/internal/gateway"
)

// turnDoneText is the notification that an agent's turn is done (itieniu
// on X), in the build's one language, Chinese: the agent by its name, how
// long the turn took, and the folder it works in when it says one.
func turnDoneText(_ string, agent string, t gateway.TurnEnd) (title, body string) {
	took := turnTook(t.Took)
	dir := gateway.TurnFolder(t.Folder)
	title, body = agent+" 已完成", "用时 "+took
	if dir != "" {
		body += " · " + dir
	}
	return title, body
}

// turnTook is a turn's length in minutes and seconds, or hours and minutes.
func turnTook(d time.Duration) string {
	s := int(d.Round(time.Second) / time.Second)
	h, m, sec := s/3600, s/60%60, s%60
	u := struct{ h, m, s string }{" 小时 ", " 分 ", " 秒"}
	trim := func(x string) string {
		if len(x) > 0 && x[len(x)-1] == ' ' {
			return x[:len(x)-1]
		}
		return x
	}
	switch {
	case h > 0:
		return fmt.Sprintf("%d%s%d%s", h, u.h, m, trim(u.m))
	case m > 0:
		return fmt.Sprintf("%d%s%d%s", m, u.m, sec, u.s)
	}
	return fmt.Sprintf("%d%s", sec, u.s)
}

// agentName is an agent's name as magpie shows it, its id for one it
// doesn't know.
func agentName(id string) string { return id }
