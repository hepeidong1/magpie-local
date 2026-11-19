package gui

import "io"

// lineless writes what a json.Encoder writes without the newline Encode
// ends each value with: Encode writes a value and its newline at once.
type lineless struct{ w io.Writer }

func (l lineless) Write(b []byte) (int, error) {
	if n := len(b); n > 0 && b[n-1] == '\n' {
		_, err := l.w.Write(b[:n-1])
		return n, err
	}
	return l.w.Write(b)
}
