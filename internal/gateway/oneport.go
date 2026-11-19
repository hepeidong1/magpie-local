package gateway

import (
	"net/http"
)

// Front is what the gateway's own listener serves: Handler behind
// lanGuard, built once.
func (s *Server) Front() http.Handler {
	// the goroutine dump ahead of lanGuard, whose key check reads the key
	// store under its lock (dumpFirst)
	// (socketEntry: a socket's turns go through all of it again)
	s.frontOnce.Do(func() { s.front = socketEntry(s.dumpFirst(lanGuard(s.Handler()))) })
	return s.front
}
