package orderstatmultiset

import (
	"errors"
	"reflect"
	"sync"
	"testing"
)

// checkInvariants verifies AVL balance, cached height/size, and BST order.
func checkInvariants(t *testing.T, m *Multiset) {
	t.Helper()
	var walk func(n *node, lo, hi int64) int
	walk = func(n *node, lo, hi int64) int {
		if n == nil {
			return 0
		}
		if n.value <= lo || n.value >= hi {
			t.Errorf("node %d outside bounds (%d, %d)", n.value, lo, hi)
		}
		ls := walk(n.left, lo, n.value)
		rs := walk(n.right, n.value, hi)
		if want := n.count + ls + rs; n.size != want {
			t.Errorf("node %d: size = %d, want %d", n.value, n.size, want)
		}
		if want := 1 + max(height(n.left), height(n.right)); n.height != want {
			t.Errorf("node %d: height = %d, want %d", n.value, n.height, want)
		}
		if bf := balanceFactor(n); bf < -1 || bf > 1 {
			t.Errorf("node %d: balance factor %d", n.value, bf)
		}
		return n.size
	}
	const maxInt64 = int64(^uint64(0) >> 1)
	if got := walk(m.root, -maxInt64-1, maxInt64); got != m.Len() {
		t.Errorf("subtree sizes sum to %d, Len() = %d", got, m.Len())
	}
}

func mustAdd(t *testing.T, m *Multiset, values ...int64) {
	t.Helper()
	for _, v := range values {
		if err := m.Add(v); err != nil {
			t.Fatalf("Add(%d): %v", v, err)
		}
	}
}

func TestAddDuplicatesAndCount(t *testing.T) {
	m := New()
	mustAdd(t, m, 5, 2, 5, 8, 2, 5)
	checkInvariants(t, m)

	if got, want := m.Count(5), 3; got != want {
		t.Errorf("Count(5) = %d, want %d", got, want)
	}
	if got, want := m.Count(2), 2; got != want {
		t.Errorf("Count(2) = %d, want %d", got, want)
	}
	if got := m.Count(99); got != 0 {
		t.Errorf("Count(99) = %d, want 0", got)
	}
	if got, want := m.Len(), 6; got != want {
		t.Errorf("Len() = %d, want %d", got, want)
	}
	if got, want := m.DistinctLen(), 3; got != want {
		t.Errorf("DistinctLen() = %d, want %d", got, want)
	}
}

func TestRankAndKth(t *testing.T) {
	m := New()
	mustAdd(t, m, 4, 1, 4, 7, 4, 1, 9)
	checkInvariants(t, m)
	// Sorted: 1 1 4 4 4 7 9

	ranks := map[int64]int{0: 0, 1: 0, 2: 2, 4: 2, 5: 5, 7: 5, 9: 6, 10: 7}
	for v, want := range ranks {
		if got := m.Rank(v); got != want {
			t.Errorf("Rank(%d) = %d, want %d", v, got, want)
		}
	}

	want := []int64{1, 1, 4, 4, 4, 7, 9}
	for k, w := range want {
		got, err := m.Kth(k)
		if err != nil || got != w {
			t.Errorf("Kth(%d) = %d, %v; want %d", k, got, err, w)
		}
	}
}

func TestRotations(t *testing.T) {
	cases := []struct {
		name     string
		values   []int64
		wantRoot int64
	}{
		{"left-left", []int64{3, 2, 1}, 2},
		{"right-right", []int64{1, 2, 3}, 2},
		{"left-right", []int64{3, 1, 2}, 2},
		{"right-left", []int64{1, 3, 2}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New()
			mustAdd(t, m, tc.values...)
			if m.root.value != tc.wantRoot {
				t.Fatalf("root = %d, want %d (rotation missing)", m.root.value, tc.wantRoot)
			}
			checkInvariants(t, m)
			if got := m.Snapshot(); !reflect.DeepEqual(got, []int64{1, 2, 3}) {
				t.Errorf("Snapshot() = %v, want [1 2 3]", got)
			}
		})
	}

	// Larger ascending/descending inserts force many rotations.
	for _, n := range []int{20, 21} {
		m := New()
		for i := 0; i < n; i++ {
			mustAdd(t, m, int64(i))
		}
		for i := n; i < 2*n; i++ {
			mustAdd(t, m, int64(3*n-i-1))
		}
		checkInvariants(t, m)
		if got := m.Len(); got != 2*n {
			t.Errorf("Len() = %d, want %d", got, 2*n)
		}
	}
}

func TestRemoveCountsAndNodeDeletion(t *testing.T) {
	m := New()
	mustAdd(t, m, 5, 3, 8, 3, 5, 1, 8, 3)
	checkInvariants(t, m)
	// Counts: 1x1, 3x3, 5x2, 8x2

	if err := m.Remove(3); err != nil {
		t.Fatal(err)
	}
	if got := m.Count(3); got != 2 {
		t.Errorf("after one Remove(3), Count(3) = %d, want 2", got)
	}
	if got := m.DistinctLen(); got != 4 {
		t.Errorf("DistinctLen() = %d, want 4 (node must stay)", got)
	}

	// Drain 3: node deleted on last occurrence.
	if err := m.Remove(3); err != nil {
		t.Fatal(err)
	}
	if err := m.Remove(3); err != nil {
		t.Fatal(err)
	}
	if got := m.Count(3); got != 0 {
		t.Errorf("Count(3) = %d, want 0", got)
	}
	if got := m.DistinctLen(); got != 3 {
		t.Errorf("DistinctLen() = %d, want 3 (node must be gone)", got)
	}
	checkInvariants(t, m)

	// Remove 5 (two children at root area) and leaf 1.
	if err := m.Remove(5); err != nil {
		t.Fatal(err)
	}
	if err := m.Remove(5); err != nil {
		t.Fatal(err)
	}
	if err := m.Remove(1); err != nil {
		t.Fatal(err)
	}
	checkInvariants(t, m)

	if got, want := m.Snapshot(), []int64{8, 8}; !reflect.DeepEqual(got, want) {
		t.Errorf("Snapshot() = %v, want %v", got, want)
	}
	if got := m.Len(); got != 2 {
		t.Errorf("Len() = %d, want 2", got)
	}
	if got := m.Rank(8); got != 0 {
		t.Errorf("Rank(8) = %d, want 0", got)
	}
}

func TestBoundaryErrors(t *testing.T) {
	m := New()

	if _, err := m.Kth(0); !errors.Is(err, ErrOutOfRange) {
		t.Errorf("Kth on empty = %v, want ErrOutOfRange", err)
	}
	if err := m.Remove(7); !errors.Is(err, ErrNotFound) {
		t.Errorf("Remove on empty = %v, want ErrNotFound", err)
	}

	mustAdd(t, m, 10, 10, 20)
	for _, k := range []int{-1, 3, 100} {
		if _, err := m.Kth(k); !errors.Is(err, ErrOutOfRange) {
			t.Errorf("Kth(%d) = %v, want ErrOutOfRange", k, err)
		}
	}
	if err := m.Remove(11); !errors.Is(err, ErrNotFound) {
		t.Errorf("Remove(11) = %v, want ErrNotFound", err)
	}
	if got := m.Len(); got != 3 {
		t.Errorf("Len() = %d after failed ops, want 3", got)
	}
}

func TestSnapshot(t *testing.T) {
	m := New()
	mustAdd(t, m, 9, 4, 9, -2, 4, 0)

	got := m.Snapshot()
	want := []int64{-2, 0, 4, 4, 9, 9}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Snapshot() = %v, want %v", got, want)
	}

	// Mutating the snapshot must not affect the multiset, and later
	// mutations must not affect the snapshot.
	got[0] = 12345
	mustAdd(t, m, 7)
	if v, err := m.Kth(0); err != nil || v != -2 {
		t.Errorf("Kth(0) = %d, %v; want -2 (snapshot not independent)", v, err)
	}
	if !reflect.DeepEqual(got, []int64{12345, 0, 4, 4, 9, 9}) {
		t.Errorf("snapshot changed after Add: %v", got)
	}
}

func TestClose(t *testing.T) {
	m := New()
	mustAdd(t, m, 3, 1, 3)
	m.Close()
	m.Close() // must be idempotent

	if err := m.Add(5); !errors.Is(err, ErrClosed) {
		t.Errorf("Add after Close = %v, want ErrClosed", err)
	}
	if err := m.Remove(3); !errors.Is(err, ErrClosed) {
		t.Errorf("Remove after Close = %v, want ErrClosed", err)
	}

	// All queries still work.
	if got := m.Len(); got != 3 {
		t.Errorf("Len() = %d, want 3", got)
	}
	if got := m.DistinctLen(); got != 2 {
		t.Errorf("DistinctLen() = %d, want 2", got)
	}
	if got := m.Count(3); got != 2 {
		t.Errorf("Count(3) = %d, want 2", got)
	}
	if got := m.Rank(3); got != 1 {
		t.Errorf("Rank(3) = %d, want 1", got)
	}
	if v, err := m.Kth(2); err != nil || v != 3 {
		t.Errorf("Kth(2) = %d, %v; want 3", v, err)
	}
	if got := m.Snapshot(); !reflect.DeepEqual(got, []int64{1, 3, 3}) {
		t.Errorf("Snapshot() = %v, want [1 3 3]", got)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	m := New()
	const writers = 4
	const perWriter = 50

	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(base int64) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				if err := m.Add(base + int64(i)); err != nil {
					t.Errorf("Add: %v", err)
				}
			}
		}(int64(w * perWriter))
	}
	// Readers hammer queries while writers work.
	for r := 0; r < 2; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				m.Rank(int64(i))
				m.Count(int64(i))
				m.Len()
				m.DistinctLen()
				m.Snapshot()
				if k := m.Len(); k > 0 {
					if _, err := m.Kth(k - 1); err != nil {
						t.Errorf("Kth(%d): %v", k-1, err)
					}
				}
			}
		}()
	}
	wg.Wait()

	if got, want := m.Len(), writers*perWriter; got != want {
		t.Fatalf("Len() = %d, want %d", got, want)
	}
	checkInvariants(t, m)

	// Concurrently remove every element; total must reach exactly zero.
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(base int64) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				if err := m.Remove(base + int64(i)); err != nil {
					t.Errorf("Remove: %v", err)
				}
			}
		}(int64(w * perWriter))
	}
	wg.Wait()

	if got := m.Len(); got != 0 {
		t.Errorf("Len() = %d after concurrent removes, want 0", got)
	}
	if got := m.DistinctLen(); got != 0 {
		t.Errorf("DistinctLen() = %d, want 0", got)
	}
	checkInvariants(t, m)
}
