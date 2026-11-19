package gateway

// Recent 环的落盘与回载（recent-calls.json）：请求页的日志原本只在
// 内存里，重启即空。环随每次调用节流落盘（0.8 秒合并一次），停服时
// 终存一次，启动时回载并续上 Call 的 ID 号。环里的调用本就不带报文
// 主体（record 已剥），落盘的也只有元数据。

import (
	"encoding/json"
	"log"
	"os"
	"slices"
	"time"

	"github.com/yetone/magpie/internal/appdir"
)

// recentSaveSoon 节流：0.8 秒内的多次记录合并成一次写（单飞）。
func (s *Server) saveRecentSoon(ring []Call) {
	s.recentMu.Lock()
	if s.recentSaving {
		s.recentAgain = true
		s.recentMu.Unlock()
		return
	}
	s.recentSaving = true
	s.recentMu.Unlock()
	time.AfterFunc(800*time.Millisecond, s.saveRecentFlush)
	_ = ring
}

// saveRecentFlush 把当前环写到 recent-calls.json；期间又来过新调用就
// 再排一次。
func (s *Server) saveRecentFlush() {
	s.mu.Lock()
	ring := slices.Clone(s.recent)
	s.mu.Unlock()
	saveRecentCalls(ring)
	s.recentMu.Lock()
	s.recentSaving = false
	again := s.recentAgain
	s.recentAgain = false
	s.recentMu.Unlock()
	if again {
		s.saveRecentSoon(nil)
	}
}

// saveRecentNow 同步写一次（停服终存）。
func (s *Server) saveRecentNow() {
	s.mu.Lock()
	ring := slices.Clone(s.recent)
	s.mu.Unlock()
	saveRecentCalls(ring)
}

func recentCallsPath() string { return appdir.Join(appdir.Config(), "recent-calls.json") }

func saveRecentCalls(ring []Call) {
	b, err := json.Marshal(ring)
	if err != nil {
		return
	}
	tmp := recentCallsPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		log.Printf("magpie: could not save the recent calls: %v", err)
		return
	}
	if err := os.Rename(tmp, recentCallsPath()); err != nil {
		log.Printf("magpie: could not save the recent calls: %v", err)
	}
}

// loadRecentCalls 回载上一次的环（旧的在前），并把 ID 号续到最大值之后。
func (s *Server) loadRecent() {
	b, err := os.ReadFile(recentCallsPath())
	if err != nil {
		return
	}
	var ring []Call
	if err := json.Unmarshal(b, &ring); err != nil {
		log.Printf("magpie: could not read the recent calls: %v", err)
		return
	}
	if len(ring) > recentMost {
		ring = ring[len(ring)-recentMost:]
	}
	s.mu.Lock()
	s.recent = ring
	for _, c := range ring {
		if c.ID > s.calls {
			s.calls = c.ID
		}
	}
	s.mu.Unlock()
}
