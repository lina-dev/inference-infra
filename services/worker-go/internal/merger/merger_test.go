package merger

import (
	"context"
	"testing"

	"github.com/inference-infra/worker/internal/triton"
)

func segChanFrom(segs []triton.Segment) <-chan triton.Segment {
	ch := make(chan triton.Segment, len(segs))
	for _, s := range segs {
		ch <- s
	}
	close(ch)
	return ch
}

// Test 6: segments produce correctly formatted timestamped lines.
func TestMerge_formatsTimestamps(t *testing.T) {
	segs := []triton.Segment{
		{ChunkIndex: 0, StartSec: 0.0, EndSec: 1.0, Text: "hello world"},
		{ChunkIndex: 1, StartSec: 1.0, EndSec: 2.0, Text: "how are you"},
	}
	got, err := Merge(context.Background(), segChanFrom(segs))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "[0.00 - 1.00] hello world.\n[1.00 - 2.00] how are you."
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// Test 7: segments with blank text are silently skipped.
func TestMerge_skipsBlankSegments(t *testing.T) {
	segs := []triton.Segment{
		{ChunkIndex: 0, StartSec: 0.0, EndSec: 1.0, Text: "hello"},
		{ChunkIndex: 1, StartSec: 1.0, EndSec: 2.0, Text: "   "}, // whitespace only
		{ChunkIndex: 2, StartSec: 2.0, EndSec: 3.0, Text: ""},    // empty
		{ChunkIndex: 3, StartSec: 3.0, EndSec: 4.0, Text: "world"},
	}
	got, err := Merge(context.Background(), segChanFrom(segs))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "[0.00 - 1.00] hello.\n[3.00 - 4.00] world."
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// Test 8: a cancelled context causes Merge to return ctx.Err() before the channel closes.
func TestMerge_contextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Unbuffered channel that never sends — Merge must unblock via ctx.
	ch := make(chan triton.Segment)
	done := make(chan struct{})

	go func() {
		defer close(done)
		_, err := Merge(ctx, ch)
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	}()

	cancel()
	<-done
}
