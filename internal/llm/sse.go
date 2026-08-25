package llm

import (
	"bufio"
	"io"
	"strings"
)

// sseReader yields the payload of each `data:` line of a text/event-stream.
// bufio.Scanner buffers internally, so chunks split across reads (or split
// mid-line by the network) are reassembled transparently; blank keep-alive
// lines and any other non-`data:` line are skipped.
type sseReader struct{ sc *bufio.Scanner }

func newSSEReader(r io.Reader) *sseReader {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	return &sseReader{sc: sc}
}

func (s *sseReader) next() (string, error) {
	for s.sc.Scan() {
		line := strings.TrimSpace(s.sc.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, "data:")), nil
	}
	if err := s.sc.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}
