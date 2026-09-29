package comm

import (
	"bytes"
	"io"
	"testing"
	"time"
)

type rwBytes struct {
	in  []byte
	out bytes.Buffer
}

func (s *rwBytes) Read(p []byte) (int, error) {
	if len(s.in) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s.in)
	s.in = s.in[n:]
	return n, nil
}
func (s *rwBytes) Write(p []byte) (int, error)      { return s.out.Write(p) }
func (s *rwBytes) Close() error                     { return nil }
func (s *rwBytes) SetReadDeadline(time.Time) error  { return nil }
func (s *rwBytes) SetWriteDeadline(time.Time) error { return nil }
func (s *rwBytes) Local() bool                      { return false }

func TestTelnetCollapsesCRLFAndCRNUL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"ab\r\ncd", "ab\rcd"},
		{"ab\r\x00cd", "ab\rcd"},
		{"ab\rcd", "ab\rcd"},
	}
	for _, c := range cases {
		tn := NewTelnet(&rwBytes{in: []byte(c.in)})
		var got []byte
		buf := make([]byte, 32)
		for {
			n, err := tn.Read(buf)
			if n > 0 {
				got = append(got, buf[:n]...)
			}
			if err != nil {
				break
			}
		}
		if string(got) != c.want {
			t.Fatalf("in %q got %q want %q", c.in, got, c.want)
		}
	}
}

type chunkStream struct {
	chunks [][]byte
	out    bytes.Buffer
}

func (s *chunkStream) Read(p []byte) (int, error) {
	if len(s.chunks) == 0 {
		return 0, io.EOF
	}
	n := copy(p, s.chunks[0])
	s.chunks[0] = s.chunks[0][n:]
	if len(s.chunks[0]) == 0 {
		s.chunks = s.chunks[1:]
	}
	return n, nil
}
func (s *chunkStream) Write(p []byte) (int, error)      { return s.out.Write(p) }
func (s *chunkStream) Close() error                     { return nil }
func (s *chunkStream) SetReadDeadline(time.Time) error  { return nil }
func (s *chunkStream) SetWriteDeadline(time.Time) error { return nil }
func (s *chunkStream) Local() bool                      { return false }

func readTelnetKeys(t *testing.T, tn *Telnet) []byte {
	t.Helper()
	var got []byte
	buf := make([]byte, 1)
	for {
		n, err := tn.Read(buf)
		if n > 0 {
			got = append(got, buf[:n]...)
		}
		if err != nil {
			return got
		}
	}
}

func TestTelnetRemembersBareCRDropsNextLForNUL(t *testing.T) {
	cases := []struct {
		chunks [][]byte
		want   string
	}{
		{[][]byte{{'\r'}, {'\n', 'X'}}, "\rX"},
		{[][]byte{{'\r'}, {0, 'X'}}, "\rX"},
		{[][]byte{{'A', '\r'}, {'\n', 'B'}}, "A\rB"},
		{[][]byte{{'\r'}, {'A'}}, "\rA"},
	}
	for _, c := range cases {
		tn := NewTelnet(&chunkStream{chunks: append([][]byte(nil), c.chunks...)})
		got := readTelnetKeys(t, tn)
		if string(got) != c.want {
			t.Fatalf("chunks %q got %q want %q", c.chunks, got, c.want)
		}
	}
}
