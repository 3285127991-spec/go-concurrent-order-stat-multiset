package multiset

import (
	"errors"
	"slices"
	"sync"
	"testing"
)

// checkInvariants verifies AVL balance, per-node height and subtree size,
// BST ordering, and that the tracked totals match a full recount.
func checkInvariants(t *testing.T, m *Multiset) {
	t.Helper()
	nodes := 0
	var walk func(n *node) (height, size int)
	walk = func(n *node) (int, int) {
		if n == nil {
			return 0, 0
		}
		nodes++
		lh, ls := walk(n.left)
		rh, rs := walk(n.right)
		if n.count <= 0 {
			t.Errorf("node %d has non-positive count %d", n.value, n.count)
		}
		if n.left != nil && n.left.value >= n.value {
			t.Errorf("BST violation: left child %d >= %d", n.left.value, n.value)
		}
		if n.right != nil && n.right.value <= n.value {
			t.Errorf("BST violation: right child %d <= %d", n.right.value, n.value)
		}
		if diff := lh - rh; diff < -1 || diff > 1 {
			t.Errorf("node %d unbalanced: left height %d, right height %d", n.value, lh, rh)
		}
		if want := 1 + max(lh, rh); n.height != want {
			t.Errorf("node %d height = %d, want %d", n.value, n.height, want)
		}
		if want := n.count + ls + rs; n.size != want {
			t.Errorf("node %d size = %d, want %d", n.value, n.size, want)
		}
		return n.height, n.size
	}
	_, total := walk(m.root)
	if total != m.total {
		t.Errorf("m.total = %d, recomputed subtree size %d", m.total, total)
	}
	if nodes != m.distinct {
		t.Errorf("m.distinct = %d, counted %d nodes", m.distinct, nodes)
	}
}

func mustAdd(t *testing.T, m *Multiset, vals ...int64) {
	t.Helper()
	for _, v := range vals {
		if err := m.Add(v); err != nil {
			t.Fatalf("Add(%d): %v", v, err)
		}
	}
}

func mustRemove(t *testing.T, m *Multiset, v int64) {
	t.Helper()
	if err := m.Remove(v); err != nil {
		t.Fatalf("Remove(%d): %v", v, err)
	}
}

func TestAddDuplicatesAndCount(t *testing.T) {
	m := New()
	mustAdd(t, m, 5, 1, 9, 1, 5, 3, 9, 9, 7)

	if got, want := m.Len(), 9; got != want {
		t.Errorf("Len() = %d, want %d", got, want)
	}
	if got, want := m.DistinctLen(), 5; got != want {
		t.Errorf("DistinctLen() = %d, want %d", got, want)
	}
	for v, want := range map[int64]int{1: 2, 5: 2, 9: 3, 3: 1, 100: 0} {
		if got := m.Count(v); got != want {
			t.Errorf("Count(%d) = %d, want %d", v, got, want)
		}
	}
	checkInvariants(t, m)
}

func TestRankAndKth(t *testing.T) {
	m := New()
	sorted := []int64{1, 1, 3, 5, 5, 7, 9, 9, 9}
	mustAdd(t, m, 9, 5, 1, 9, 3, 5, 9, 1, 7)

	for v, want := range map[int64]int{0: 0, 1: 0, 2: 2, 3: 2, 5: 3, 9: 6, 10: 9} {
		if got := m.Rank(v); got != want {
			t.Errorf("Rank(%d) = %d, want %d", v, got, want)
		}
	}
	for k, want := range sorted {
		got, err := m.Kth(k)
		if err != nil {
			t.Fatalf("Kth(%d): %v", k, err)
		}
		if got != want {
			t.Errorf("Kth(%d) = %d, want %d", k, got, want)
		}
	}
	checkInvariants(t, m)
}

func TestRotations(t *testing.T) {
	cases := []struct {
		name string
		vals []int64
	}{
		{"left-left", []int64{3, 2, 1}},
		{"right-right", []int64{1, 2, 3}},
		{"left-right", []int64{3, 1, 2}},
		{"right-left", []int64{1, 3, 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New()
			mustAdd(t, m, tc.vals...)
			mustAdd(t, m, 2, 2) // duplicates must not disturb balance metadata
			if m.root == nil || m.root.value != 2 {
				t.Fatalf("root = %v, want 2 after rotation", m.root)
			}
			if got, want := m.Snapshot(), []int64{1, 2, 2, 2, 3}; !slices.Equal(got, want) {
				t.Errorf("Snapshot() = %v, want %v", got, want)
			}
			checkInvariants(t, m)
		})
	}
}

func TestRemoveNodeAndCount(t *testing.T) {
	m := New()
	mustAdd(t, m, 5, 5, 5, 3, 7, 1, 9)

	mustRemove(t, m, 5) // count 3 -> 2, node stays
	if got := m.Count(5); got != 2 {
		t.Errorf("Count(5) = %d, want 2", got)
	}
	if got := m.DistinctLen(); got != 5 {
		t.Errorf("DistinctLen() = %d, want 5 (node must stay)", got)
	}
	mustRemove(t, m, 5)
	mustRemove(t, m, 5) // last occurrence: node removed
	if got := m.Count(5); got != 0 {
		t.Errorf("Count(5) = %d, want 0", got)
	}
	if got := m.DistinctLen(); got != 4 {
		t.Errorf("DistinctLen() = %d, want 4", got)
	}
	mustRemove(t, m, 3)
	if got, want := m.Snapshot(), []int64{1, 7, 9}; !slices.Equal(got, want) {
		t.Errorf("Snapshot() = %v, want %v", got, want)
	}
	checkInvariants(t, m)

	// Removing a node with two children splices in its successor.
	m2 := New()
	mustAdd(t, m2, 4, 2, 6, 1, 3, 5, 7)
	mustRemove(t, m2, 4)
	if got, want := m2.Snapshot(), []int64{1, 2, 3, 5, 6, 7}; !slices.Equal(got, want) {
		t.Errorf("Snapshot() = %v, want %v", got, want)
	}
	mustRemove(t, m2, 1) // leaf
	if got, want := m2.Snapshot(), []int64{2, 3, 5, 6, 7}; !slices.Equal(got, want) {
		t.Errorf("Snapshot() = %v, want %v", got, want)
	}
	checkInvariants(t, m2)
}

func TestNotFoundAndOutOfRange(t *testing.T) {
	m := New()
	if err := m.Remove(1); !errors.Is(err, ErrNotFound) {
		t.Errorf("Remove on empty = %v, want ErrNotFound", err)
	}
	for _, k := range []int{-1, 0} {
		if _, err := m.Kth(k); !errors.Is(err, ErrOutOfRange) {
			t.Errorf("Kth(%d) on empty = %v, want ErrOutOfRange", k, err)
		}
	}

	mustAdd(t, m, 1, 2, 2)
	if err := m.Remove(3); !errors.Is(err, ErrNotFound) {
		t.Errorf("Remove(3) = %v, want ErrNotFound", err)
	}
	for _, k := range []int{-1, 3, 100} {
		if _, err := m.Kth(k); !errors.Is(err, ErrOutOfRange) {
			t.Errorf("Kth(%d) = %v, want ErrOutOfRange", k, err)
		}
	}
	if _, err := m.Kth(2); err != nil {
		t.Errorf("Kth(2) = %v, want nil", err)
	}
}

func TestSnapshot(t *testing.T) {
	m := New()
	mustAdd(t, m, 9, 5, 1, 9, 3, 5, 9, 1, 7)

	want := []int64{1, 1, 3, 5, 5, 7, 9, 9, 9}
	snap := m.Snapshot()
	if !slices.Equal(snap, want) {
		t.Errorf("Snapshot() = %v, want %v", snap, want)
	}
	if !slices.IsSorted(snap) {
		t.Errorf("Snapshot() not sorted: %v", snap)
	}

	// The snapshot must be an independent copy.
	snap[0] = -999
	if got, err := m.Kth(0); err != nil || got != 1 {
		t.Errorf("Kth(0) = %d, %v after mutating snapshot; want 1, nil", got, err)
	}
	if got := m.Count(-999); got != 0 {
		t.Errorf("Count(-999) = %d, want 0", got)
	}
}

func TestClose(t *testing.T) {
	m := New()
	mustAdd(t, m, 2, 1, 2)
	m.Close()
	m.Close() // must be idempotent

	if err := m.Add(3); !errors.Is(err, ErrClosed) {
		t.Errorf("Add after Close = %v, want ErrClosed", err)
	}
	if err := m.Remove(1); !errors.Is(err, ErrClosed) {
		t.Errorf("Remove after Close = %v, want ErrClosed", err)
	}

	// All queries keep working after Close.
	if got := m.Count(2); got != 2 {
		t.Errorf("Count(2) = %d, want 2", got)
	}
	if got := m.Rank(2); got != 1 {
		t.Errorf("Rank(2) = %d, want 1", got)
	}
	if got, err := m.Kth(0); err != nil || got != 1 {
		t.Errorf("Kth(0) = %d, %v, want 1, nil", got, err)
	}
	if got, want := m.Snapshot(), []int64{1, 2, 2}; !slices.Equal(got, want) {
		t.Errorf("Snapshot() = %v, want %v", got, want)
	}
	if got := m.Len(); got != 3 {
		t.Errorf("Len() = %d, want 3", got)
	}
	if got := m.DistinctLen(); got != 2 {
		t.Errorf("DistinctLen() = %d, want 2", got)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	m := New()
	const writers = 4
	const perWriter = 50

	var writersWG, readersWG sync.WaitGroup
	stop := make(chan struct{})
	for r := 0; r < 2; r++ {
		readersWG.Add(1)
		go func() {
			defer readersWG.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = m.Len()
				_ = m.DistinctLen()
				_ = m.Rank(25)
				_ = m.Count(7)
				_ = m.Snapshot()
				_, _ = m.Kth(0) // may fail while empty; that is fine
			}
		}()
	}
	for w := 0; w < writers; w++ {
		writersWG.Add(1)
		go func(base int64) {
			defer writersWG.Done()
			for i := int64(0); i < perWriter; i++ {
				v := base*perWriter + i
				_ = m.Add(v)
				_ = m.Add(v) // duplicate
			}
			for i := int64(0); i < perWriter; i++ {
				_ = m.Remove(base*perWriter + i) // drop one occurrence
			}
		}(int64(w))
	}
	writersWG.Wait()
	close(stop)
	readersWG.Wait()

	// Every value was added twice and removed once: count 1 each.
	if got, want := m.Len(), writers*perWriter; got != want {
		t.Errorf("Len() = %d, want %d", got, want)
	}
	if got, want := m.DistinctLen(), writers*perWriter; got != want {
		t.Errorf("DistinctLen() = %d, want %d", got, want)
	}
	snap := m.Snapshot()
	if !slices.IsSorted(snap) || len(snap) != m.Len() {
		t.Errorf("Snapshot() inconsistent: len %d, sorted %v", len(snap), slices.IsSorted(snap))
	}
	checkInvariants(t, m)
}
