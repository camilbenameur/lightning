package sandbox

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

type chunk struct {
	stream Stream
	data   string
}

func (c chunk) String() string { return c.stream.String() + ":" + strconv.Quote(c.data) }

// recordingSink records events; it copies data as the Sink contract requires.
type recordingSink struct {
	started int
	chunks  []chunk
}

func (r *recordingSink) Started() { r.started++ }
func (r *recordingSink) Output(s Stream, p []byte) {
	r.chunks = append(r.chunks, chunk{s, string(p)})
}

func (r *recordingSink) text(s Stream) string {
	var b strings.Builder
	for _, c := range r.chunks {
		if c.stream == s {
			b.WriteString(c.data)
		}
	}
	return b.String()
}

func TestCompleteRunes(t *testing.T) {
	euro := "€"  // 3 bytes
	emoji := "😀" // 4 bytes
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"abc", 3},
		{"a" + euro, 4},
		{"a" + euro[:1], 1},
		{"a" + euro[:2], 1},
		{emoji, 4},
		{"xy" + emoji[:3], 2},
		{"é"[:1], 0},
		{"\xff", 1},              // invalid byte: nothing to wait for
		{"a\x80\x80\x80\x80", 5}, // stray continuation bytes
		{euro[1:], 2},            // continuation bytes without a start
	}
	for _, tt := range tests {
		if got := completeRunes([]byte(tt.in)); got != tt.want {
			t.Errorf("completeRunes(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestOutputNeverSplitsRunes(t *testing.T) {
	text := "héllo wörld € 😀 日本語\n"
	for size := 1; size <= 5; size++ {
		sink := &recordingSink{}
		out := newOutput(sink, 1<<20)
		w := out.writer(Stdout)
		for p := []byte(text); len(p) > 0; {
			n := min(size, len(p))
			if _, err := w.Write(p[:n]); err != nil {
				t.Fatal(err)
			}
			p = p[n:]
		}
		out.flush()
		for _, c := range sink.chunks {
			if !utf8.ValidString(c.data) {
				t.Fatalf("write size %d: chunk %q is not valid UTF-8", size, c.data)
			}
		}
		if got := sink.text(Stdout); got != text {
			t.Fatalf("write size %d: got %q, want %q", size, got, text)
		}
	}
}

func TestOutputKeepsStreamsApart(t *testing.T) {
	sink := &recordingSink{}
	out := newOutput(sink, 1<<20)
	euro := []byte("€")
	_, _ = out.writer(Stdout).Write(euro[:1])
	_, _ = out.writer(Stderr).Write([]byte("err"))
	_, _ = out.writer(Stdout).Write(euro[1:])
	want := []chunk{{Stderr, "err"}, {Stdout, "€"}}
	if len(sink.chunks) != 2 || sink.chunks[0] != want[0] || sink.chunks[1] != want[1] {
		t.Fatalf("chunks = %v, want %v", sink.chunks, want)
	}
}

func TestOutputLimitIsCombined(t *testing.T) {
	sink := &recordingSink{}
	out := newOutput(sink, 10)
	if _, err := out.writer(Stdout).Write([]byte("123456")); err != nil {
		t.Fatal(err)
	}
	n, err := out.writer(Stderr).Write([]byte("abcdef"))
	if !errors.Is(err, errOutputLimit) || n != 6 {
		t.Fatalf("Write = %d, %v; want 6, errOutputLimit", n, err)
	}
	if !out.truncated {
		t.Fatal("truncated flag not set")
	}
	if got := sink.text(Stdout) + sink.text(Stderr); got != "123456abcd" {
		t.Fatalf("output = %q", got)
	}
	if _, err := out.writer(Stdout).Write([]byte("x")); !errors.Is(err, errOutputLimit) {
		t.Fatalf("write after truncation: %v", err)
	}
}

func TestOutputExactlyAtLimitIsNotTruncated(t *testing.T) {
	sink := &recordingSink{}
	out := newOutput(sink, 4)
	if _, err := out.writer(Stdout).Write([]byte("abcd")); err != nil {
		t.Fatal(err)
	}
	if out.truncated {
		t.Fatal("output that fits the limit must not be truncated")
	}
}

func TestOutputLimitDoesNotSplitRunes(t *testing.T) {
	sink := &recordingSink{}
	out := newOutput(sink, 6)
	_, err := out.writer(Stdout).Write([]byte("ab€€")) // limit falls inside the second €
	if !errors.Is(err, errOutputLimit) {
		t.Fatalf("err = %v", err)
	}
	out.flush()
	if got := sink.text(Stdout); got != "ab€" {
		t.Fatalf("output = %q, want %q", got, "ab€")
	}

	sink = &recordingSink{}
	out = newOutput(sink, 4)
	_, _ = out.writer(Stdout).Write([]byte("ab€"))
	out.flush()
	if got := sink.text(Stdout); got != "ab" {
		t.Fatalf("output = %q, want %q", got, "ab")
	}
}

func TestOutputFlushEmitsInvalidTail(t *testing.T) {
	sink := &recordingSink{}
	out := newOutput(sink, 100)
	_, _ = out.writer(Stderr).Write([]byte("x\xe2\x82"))
	if got := sink.text(Stderr); got != "x" {
		t.Fatalf("before flush = %q", got)
	}
	out.flush()
	if got := sink.text(Stderr); got != "x\xe2\x82" {
		t.Fatalf("after flush = %q", got)
	}
}

func FuzzOutput(f *testing.F) {
	f.Add([]byte("hello, 世界 😀"), uint8(3), uint16(8))
	f.Add([]byte("\xff\xfe€"), uint8(1), uint16(1000))
	f.Fuzz(func(t *testing.T, data []byte, size uint8, limit uint16) {
		sink := &recordingSink{}
		out := newOutput(sink, int(limit))
		w := out.writer(Stdout)
		step := int(size%16) + 1
		for p := data; len(p) > 0; {
			n := min(step, len(p))
			if _, err := w.Write(p[:n]); err != nil {
				break
			}
			p = p[n:]
		}
		out.flush()

		got := []byte(sink.text(Stdout))
		if len(got) > int(limit) {
			t.Fatalf("output %d bytes exceeds limit %d", len(got), limit)
		}
		if !bytes.HasPrefix(data, got) {
			t.Fatalf("output %q is not a prefix of input %q", got, data)
		}
		if len(data) <= int(limit) && !bytes.Equal(got, data) {
			t.Fatalf("output %q lost data from %q", got, data)
		}
		if utf8.Valid(data) {
			for _, c := range sink.chunks {
				if !utf8.ValidString(c.data) {
					t.Fatalf("chunk %q splits a rune", c.data)
				}
			}
		}
	})
}
