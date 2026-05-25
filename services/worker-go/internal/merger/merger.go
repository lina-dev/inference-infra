package merger

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/inference-infra/worker/internal/triton"
)

// Merge drains the segment channel in arrival order (which is guaranteed to be
// ChunkIndex order by the triton reorder buffer) and builds a timestamped transcript.
// Returns when the channel is closed or ctx is cancelled.
func Merge(ctx context.Context, segments <-chan triton.Segment) (string, error) {
	var b strings.Builder
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case seg, ok := <-segments:
			if !ok {
				return strings.TrimRight(b.String(), "\n"), nil
			}
			text := normalize(seg.Text)
			if text == "" {
				continue
			}
			fmt.Fprintf(&b, "[%.2f - %.2f] %s\n", seg.StartSec, seg.EndSec, text)
		}
	}
}

func normalize(s string) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if s == "" {
		return ""
	}
	if !unicode.IsPunct(rune(s[len(s)-1])) {
		s += "."
	}
	return s
}
