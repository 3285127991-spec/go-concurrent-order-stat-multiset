// Package multiset implements a concurrent order-statistic multiset for
// int64 values backed by an AVL tree. Each node stores a duplicate count
// and its subtree size so Add, Remove, Count, Rank and Kth all run in
// O(log n) time.
package multiset

import (
	"errors"
	"fmt"
	"sync"
)

var (
	// ErrClosed is returned by Add and Remove after Close has been called.
	ErrClosed = errors.New("multiset: closed")
	// ErrNotFound is returned by Remove when the value is not present.
	ErrNotFound = errors.New("multiset: value not found")
	// ErrOutOfRange is returned by Kth when k is outside [0, Len()).
	ErrOutOfRange = errors.New("multiset: index out of range")
)

type node struct {
	value  int64
	count  int // number of occurrences of value
	size   int // total elements in this subtree, counting duplicates
	height int
	left   *node
	right  *node
}

func height(n *node) int {
	if n == nil {
		return 0
	}
	return n.height
}

func size(n *node) int {
	if n == nil {
		return 0
	}
	return n.size
}

// recalc restores the height and subtree-size invariants from the
// (already valid) children. It must be called after any structural or
// count change below n.
func (n *node) recalc() {
	n.height = 1 + max(height(n.left), height(n.right))
	n.size = n.count + size(n.left) + size(n.right)
}

func balanceFactor(n *node) int {
	return height(n.left) - height(n.right)
}

func rotateLeft(y *node) *node {
	x := y.right
	t2 := x.left
	x.left = y
	y.right = t2
	y.recalc()
	x.recalc()
	return x
}

func rotateRight(y *node) *node {
	x := y.left
	t2 := x.right
	x.right = y
	y.left = t2
	y.recalc()
	x.recalc()
	return x
}

func rebalance(n *node) *node {
	n.recalc()
	switch bf := balanceFactor(n); {
	case bf > 1:
		if balanceFactor(n.left) < 0 {
			n.left = rotateLeft(n.left)
		}
		return rotateRight(n)
	case bf < -1:
		if balanceFactor(n.right) > 0 {
			n.right = rotateRight(n.right)
		}
		return rotateLeft(n)
	}
	return n
}

// insert adds one occurrence of v and reports whether a new node was
// created (false when v already existed and only its count grew).
func insert(n *node, v int64) (*node, bool) {
	if n == nil {
		return &node{value: v, count: 1, size: 1, height: 1}, true
	}
	var created bool
	switch {
	case v < n.value:
		n.left, created = insert(n.left, v)
	case v > n.value:
		n.right, created = insert(n.right, v)
	default:
		n.count++
		n.recalc()
		return n, false
	}
	return rebalance(n), created
}

// deleteMin detaches the minimum node of the subtree and returns the new
// subtree root together with the detached node.
func deleteMin(n *node) (*node, *node) {
	if n.left == nil {
		return n.right, n
	}
	var min *node
	n.left, min = deleteMin(n.left)
	return rebalance(n), min
}

// removeOne removes one occurrence of v, which must exist. It reports
// whether the node itself was removed (its last occurrence deleted).
func removeOne(n *node, v int64) (*node, bool) {
	var removedNode bool
	switch {
	case v < n.value:
		n.left, removedNode = removeOne(n.left, v)
	case v > n.value:
		n.right, removedNode = removeOne(n.right, v)
	default:
		if n.count > 1 {
			n.count--
			n.recalc()
			return n, false
		}
		if n.left == nil {
			return n.right, true
		}
		if n.right == nil {
			return n.left, true
		}
		var succ *node
		n.right, succ = deleteMin(n.right)
		succ.left, succ.right = n.left, n.right
		return rebalance(succ), true
	}
	return rebalance(n), removedNode
}

func find(n *node, v int64) *node {
	for n != nil {
		switch {
		case v < n.value:
			n = n.left
		case v > n.value:
			n = n.right
		default:
			return n
		}
	}
	return nil
}

// Multiset is a concurrent ordered multiset of int64 values with
// order-statistic queries. The zero value is ready to use.
type Multiset struct {
	mu       sync.RWMutex
	root     *node
	total    int // elements including duplicates
	distinct int // nodes (distinct values)
	closed   bool
}

// New returns an empty Multiset.
func New() *Multiset {
	return &Multiset{}
}

// Add inserts one occurrence of v. It fails with ErrClosed after Close.
func (m *Multiset) Add(v int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	var created bool
	m.root, created = insert(m.root, v)
	if created {
		m.distinct++
	}
	m.total++
	return nil
}

// Remove deletes one occurrence of v. It fails with ErrClosed after
// Close and with ErrNotFound when v is not present.
func (m *Multiset) Remove(v int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	if find(m.root, v) == nil {
		return ErrNotFound
	}
	var removedNode bool
	m.root, removedNode = removeOne(m.root, v)
	if removedNode {
		m.distinct--
	}
	m.total--
	return nil
}

// Count returns how many times v occurs in the multiset.
func (m *Multiset) Count(v int64) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if n := find(m.root, v); n != nil {
		return n.count
	}
	return 0
}

// Rank returns the number of elements strictly less than v.
func (m *Multiset) Rank(v int64) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rank := 0
	for n := m.root; n != nil; {
		if v <= n.value {
			n = n.left
		} else {
			rank += size(n.left) + n.count
			n = n.right
		}
	}
	return rank
}

// Kth returns the k-th smallest element (0-based); duplicates count
// individually. It fails with ErrOutOfRange when k is outside
// [0, Len()).
func (m *Multiset) Kth(k int) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if k < 0 || k >= m.total {
		return 0, fmt.Errorf("%w: k=%d, len=%d", ErrOutOfRange, k, m.total)
	}
	for n := m.root; n != nil; {
		left := size(n.left)
		switch {
		case k < left:
			n = n.left
		case k < left+n.count:
			return n.value, nil
		default:
			k -= left + n.count
			n = n.right
		}
	}
	// Unreachable while the size invariant holds.
	return 0, fmt.Errorf("%w: k=%d, len=%d", ErrOutOfRange, k, m.total)
}

// Snapshot returns an independent ascending copy of all elements,
// duplicates included.
func (m *Multiset) Snapshot() []int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]int64, 0, m.total)
	var walk func(n *node)
	walk = func(n *node) {
		if n == nil {
			return
		}
		walk(n.left)
		for i := 0; i < n.count; i++ {
			out = append(out, n.value)
		}
		walk(n.right)
	}
	walk(m.root)
	return out
}

// Len returns the total number of elements, duplicates included.
func (m *Multiset) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.total
}

// DistinctLen returns the number of distinct values.
func (m *Multiset) DistinctLen() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.distinct
}

// Close marks the multiset closed: Add and Remove start failing with
// ErrClosed while all queries keep working. It is safe to call Close
// multiple times.
func (m *Multiset) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
}
