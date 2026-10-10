package sandbox

import (
	"errors"
	"io"
	"slices"
	"unicode/utf8"
)

// errOutputLimit stops output copying once the combined limit is reached.
var errOutputLimit = errors.New("output limit exceeded")

// output forwards program output to a Sink. It enforces a byte limit shared by
// stdout and stderr and holds back incomplete trailing UTF-8 sequences so that
// no chunk ends in the middle of a rune.
type output struct {
	sink      Sink
	remaining int
	truncated bool
	carry     [2][]byte // incomplete trailing rune, per stream
}

func newOutput(sink Sink, limit int) *output {
	return &output{sink: sink, remaining: limit}
}

// writer returns an io.Writer feeding stream s.
func (o *output) writer(s Stream) io.Writer {
	return streamWriter{o: o, s: s}
}

type streamWriter struct {
	o *output
	s Stream
}

func (w streamWriter) Write(p []byte) (int, error) { return w.o.write(w.s, p) }

func (o *output) write(s Stream, p []byte) (int, error) {
	if o.truncated {
		return 0, errOutputLimit
	}
	n := len(p)
	if len(p) > o.remaining {
		p = p[:o.remaining]
		o.truncated = true
	}
	o.remaining -= len(p)

	buf := p
	if len(o.carry[s]) > 0 {
		buf = slices.Concat(o.carry[s], p)
	}
	end := completeRunes(buf)
	if end > 0 {
		o.sink.Output(s, buf[:end])
	}
	if o.truncated {
		// Whatever is left was cut by the limit, not by the program.
		o.carry[s] = nil
		return n, errOutputLimit
	}
	o.carry[s] = append(o.carry[s][:0], buf[end:]...)
	return n, nil
}

// flush emits bytes held back at the end of the streams. They can only be an
// invalid encoding at this point; the consumer decides how to render them.
func (o *output) flush() {
	for s, rest := range o.carry {
		if len(rest) > 0 {
			o.sink.Output(Stream(s), rest)
			o.carry[s] = nil
		}
	}
}

// completeRunes returns the length of the longest prefix of p that does not
// end with an incomplete UTF-8 sequence.
func completeRunes(p []byte) int {
	for i := len(p) - 1; i >= 0 && i >= len(p)-utf8.UTFMax; i-- {
		if utf8.RuneStart(p[i]) {
			if utf8.FullRune(p[i:]) {
				return len(p)
			}
			return i
		}
	}
	return len(p)
}
