package history

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

// makeRecord creates a RequestRecord with a unique ID and a recognisable
// model name so tests can assert ordering by ID rather than relying on
// pointer equality.
func makeRecord(id int) RequestRecord {
	return RequestRecord{
		ID:        strconv.Itoa(id),
		Model:     "model-" + strconv.Itoa(id),
		StartTime: time.Unix(int64(id), 0),
	}
}

func TestNew_DefaultMaxRecords(t *testing.T) {
	h := New(0)
	if h.cap != defaultMaxRecords {
		t.Errorf("New(0).cap = %d, want %d", h.cap, defaultMaxRecords)
	}
	if h.Len() != 0 {
		t.Errorf("New(0).Len() = %d, want 0", h.Len())
	}
}

func TestNew_NegativeMaxRecords(t *testing.T) {
	h := New(-5)
	if h.cap != defaultMaxRecords {
		t.Errorf("New(-5).cap = %d, want %d", h.cap, defaultMaxRecords)
	}
}

func TestNew_ExplicitMaxRecords(t *testing.T) {
	h := New(10)
	if h.cap != 10 {
		t.Errorf("New(10).cap = %d, want 10", h.cap)
	}
	// Underlying storage should be preallocated to capacity.
	if got := len(h.records); got != 10 {
		t.Errorf("len(records) = %d, want 10", got)
	}
}

func TestAdd_BelowCapacity(t *testing.T) {
	h := New(5)
	for i := 0; i < 3; i++ {
		h.Add(makeRecord(i))
	}
	if h.Len() != 3 {
		t.Errorf("Len() = %d, want 3", h.Len())
	}
}

func TestAdd_ExceedsCapacity_EvictsOldest(t *testing.T) {
	h := New(3)
	// Add 5 records; only the last 3 should remain.
	for i := 0; i < 5; i++ {
		h.Add(makeRecord(i))
	}
	if h.Len() != 3 {
		t.Errorf("Len() = %d, want 3 (capacity)", h.Len())
	}
	got := h.Last(0) // 0 = return all
	if len(got) != 3 {
		t.Fatalf("Last(0) returned %d records, want 3", len(got))
	}
	// Newest-first order: most recent first.
	wantOrder := []string{"4", "3", "2"}
	for i, r := range got {
		if r.ID != wantOrder[i] {
			t.Errorf("record %d: ID = %q, want %q", i, r.ID, wantOrder[i])
		}
	}
}

func TestLast_NewestFirst(t *testing.T) {
	h := New(10)
	for i := 0; i < 5; i++ {
		h.Add(makeRecord(i))
	}
	got := h.Last(5)
	if len(got) != 5 {
		t.Fatalf("Last(5) returned %d, want 5", len(got))
	}
	// Should be reverse of insertion order.
	for i, r := range got {
		wantID := strconv.Itoa(4 - i)
		if r.ID != wantID {
			t.Errorf("Last[%d].ID = %q, want %q", i, r.ID, wantID)
		}
	}
}

func TestLast_NLargerThanCount(t *testing.T) {
	h := New(10)
	for i := 0; i < 3; i++ {
		h.Add(makeRecord(i))
	}
	// n=100 > count(3) should clamp to count.
	got := h.Last(100)
	if len(got) != 3 {
		t.Errorf("Last(100) returned %d, want 3 (clamped to count)", len(got))
	}
}

func TestLast_ZeroOrNegative_ReturnsAll(t *testing.T) {
	h := New(10)
	for i := 0; i < 4; i++ {
		h.Add(makeRecord(i))
	}
	for _, n := range []int{0, -1, -100} {
		got := h.Last(n)
		if len(got) != 4 {
			t.Errorf("Last(%d) returned %d, want 4", n, len(got))
		}
	}
}

func TestLast_Empty(t *testing.T) {
	h := New(10)
	got := h.Last(5)
	if len(got) != 0 {
		t.Errorf("Last(5) on empty history returned %d, want 0", len(got))
	}
}

func TestLast_AfterWraparound_PreservesOrder(t *testing.T) {
	// Fill and overflow multiple times to stress the modular arithmetic.
	h := New(3)
	for i := 0; i < 10; i++ {
		h.Add(makeRecord(i))
	}
	// After 10 inserts into a 3-slot ring: {7, 8, 9} (slot values, head=1).
	got := h.Last(3)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	wantIDs := []string{"9", "8", "7"}
	for i, r := range got {
		if r.ID != wantIDs[i] {
			t.Errorf("[%d].ID = %q, want %q", i, r.ID, wantIDs[i])
		}
	}
}

func TestLast_PartialAfterWraparound(t *testing.T) {
	// After exactly 5 inserts into 3-slot ring, Last(2) should be {4, 3}.
	h := New(3)
	for i := 0; i < 5; i++ {
		h.Add(makeRecord(i))
	}
	got := h.Last(2)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != "4" || got[1].ID != "3" {
		t.Errorf("IDs = [%q, %q], want [\"4\", \"3\"]", got[0].ID, got[1].ID)
	}
}

func TestLen_TracksCount(t *testing.T) {
	h := New(4)
	if h.Len() != 0 {
		t.Errorf("initial Len() = %d, want 0", h.Len())
	}
	for i := 0; i < 3; i++ {
		h.Add(makeRecord(i))
	}
	if h.Len() != 3 {
		t.Errorf("after 3 adds Len() = %d, want 3", h.Len())
	}
	for i := 3; i < 7; i++ {
		h.Add(makeRecord(i))
	}
	// After 4 more adds we are well past capacity; count should plateau at 4.
	if h.Len() != 4 {
		t.Errorf("after wraparound Len() = %d, want 4", h.Len())
	}
}

func TestLast_ReturnsCopies_NotInternalReferences(t *testing.T) {
	// Mutating a returned record must not affect the next call to Last.
	h := New(3)
	h.Add(makeRecord(1))
	got := h.Last(1)
	got[0].Model = "tampered"
	got2 := h.Last(1)
	if got2[0].Model != "model-1" {
		t.Errorf("internal record was mutated; got Model = %q", got2[0].Model)
	}
}

func TestConcurrent_AddAndLast(t *testing.T) {
	// Run with -race to catch any data races in the ring buffer.
	h := New(50)
	// Use separate WaitGroups for writers and readers so we can wait for
	// writers to finish before signalling readers to stop. Sharing a single
	// wg (and closing stop only after wg.Wait) would deadlock, because
	// readers only exit when stop is closed.
	var writerWG, readerWG sync.WaitGroup
	const writers = 10
	const writesEach = 200
	for w := 0; w < writers; w++ {
		writerWG.Add(1)
		go func(base int) {
			defer writerWG.Done()
			for i := 0; i < writesEach; i++ {
				h.Add(makeRecord(base*writesEach + i))
			}
		}(w)
	}
	// Concurrent readers while writers are running.
	const readers = 4
	stop := make(chan struct{})
	for r := 0; r < readers; r++ {
		readerWG.Add(1)
		go func() {
			defer readerWG.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = h.Last(10)
				}
			}
		}()
	}
	// Wait for writers to finish, then signal readers to stop and join them.
	writerWG.Wait()
	close(stop)
	readerWG.Wait()
	// Sanity check: Len should equal capacity.
	if h.Len() != 50 {
		t.Errorf("after %d writers × %d writes into cap=50, Len = %d, want 50",
			writers, writesEach, h.Len())
	}
}
