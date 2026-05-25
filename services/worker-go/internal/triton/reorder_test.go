package triton

import (
	"testing"
)

// Test 1: consecutive segments are emitted immediately.
func TestReorderBuffer_inOrder(t *testing.T) {
	out := make(chan Segment, 10)
	buf := &reorderBuffer{out: out}

	for i := range 5 {
		buf.push(Segment{ChunkIndex: i})
	}
	if len(out) != 5 {
		t.Fatalf("expected 5 emitted, got %d", len(out))
	}
	for i := range 5 {
		if seg := <-out; seg.ChunkIndex != i {
			t.Errorf("position %d: ChunkIndex = %d", i, seg.ChunkIndex)
		}
	}
}

// Test 2: a gap holds later segments until the missing one arrives, then flushes the run.
func TestReorderBuffer_outOfOrder(t *testing.T) {
	out := make(chan Segment, 10)
	buf := &reorderBuffer{out: out}

	buf.push(Segment{ChunkIndex: 0})
	if len(out) != 1 {
		t.Fatalf("after 0: want 1 emitted, got %d", len(out))
	}
	buf.push(Segment{ChunkIndex: 2}) // gap at 1 — must not emit
	if len(out) != 1 {
		t.Fatalf("after 2 (gap): want 1 emitted, got %d", len(out))
	}
	buf.push(Segment{ChunkIndex: 1}) // fills gap — 1 and 2 flush
	if len(out) != 3 {
		t.Fatalf("after 1 fills gap: want 3 emitted, got %d", len(out))
	}
	for i := range 3 {
		if seg := <-out; seg.ChunkIndex != i {
			t.Errorf("position %d: ChunkIndex = %d", i, seg.ChunkIndex)
		}
	}
}

// Test 3: heap size never exceeds the number of in-flight chunks (≤ concurrency).
func TestReorderBuffer_heapBoundedByConcurrency(t *testing.T) {
	const concurrency = 4
	out := make(chan Segment, concurrency*2)
	buf := &reorderBuffer{out: out}

	// Worst case: last concurrency chunks arrive before chunk 0.
	for i := concurrency; i >= 1; i-- {
		buf.push(Segment{ChunkIndex: i})
		if buf.pending.Len() > concurrency {
			t.Errorf("heap size %d exceeds concurrency %d", buf.pending.Len(), concurrency)
		}
	}
	buf.push(Segment{ChunkIndex: 0}) // fills gap — everything flushes
	if buf.pending.Len() != 0 {
		t.Errorf("heap not empty after flush: %d items remain", buf.pending.Len())
	}
	if len(out) != concurrency+1 {
		t.Errorf("expected %d emitted, got %d", concurrency+1, len(out))
	}
}
