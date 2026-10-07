// Package orderstatmultiset implements a concurrent order-statistic multiset
// backed by an AVL tree. Each node stores a duplicate count and its subtree
// size, so Add, Remove, Count, Rank and Kth all run in O(log n).
package orderstatmultiset

import (
	"errors"
	"fmt"
	"sync"
)

var (
	// ErrNotFound is returned by Remove when the value is not present.
	ErrNotFound = errors.New("value not found")
	// ErrOutOfRange is returned by Kth when k is outside [0, Len()).
	ErrOutOfRange = errors.New("index out of range")
	// ErrClosed is returned by Add and Remove after Close.
	ErrClosed = errors.New("closed for writes")
)

type node struct {
	value  int64
	count  int // occurrences of value at this node
	size   int // total elements in this subtree, counting duplicates
	height int
	left   *node
	right  *node
}

func size(n *node) int {
	if n == nil {
		return 0
	}
	return n.size
}

func height(n *node) int {
	if n == nil {
		return 0
	}
	return n.height
}

// recalc restores the size and height of n from its children.
func (n *node) recalc() {
	n.size = n.count + size(n.left) + size(n.right)
	n.height = 1 + max(height(n.left), height(n.right))
}

func balanceFactor(n *node) int {
	return height(n.left) - height(n.right)
}

func rotateRight(y *node) *node {
	x := y.left
	x.right, y.left = y, x.right
	y.recalc()
	x.recalc()
	return x
}

func rotateLeft(x *node) *node {
	y := x.right
	y.left, x.right = x, y.left
	x.recalc()
	y.recalc()
	return y
}

// rebalance recalculates n's metadata and rotates if needed.
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

// insert adds value to the subtree and reports whether a new node was created.
func insert(n *node, value int64) (*node, bool) {
	if n == nil {
		return &node{value: value, count: 1, size: 1, height: 1}, true
	}
	created := false
	switch {
	case value < n.value:
		n.left, created = insert(n.left, value)
	case value > n.value:
		n.right, created = insert(n.right, value)
	default:
		n.count++
	}
	return rebalance(n), created
}

// removeMin detaches the minimum node of the subtree, returning the new
// subtree root and the detached node.
func removeMin(n *node) (*node, *node) {
	if n.left == nil {
		return n.right, n
	}
	var min *node
	n.left, min = removeMin(n.left)
	return rebalance(n), min
}

// remove deletes one occurrence of value from the subtree and reports
// whether the value was found.
func remove(n *node, value int64) (*node, bool) {
	if n == nil {
		return nil, false
	}
	found := true
	switch {
	case value < n.value:
		n.left, found = remove(n.left, value)
	case value > n.value:
		n.right, found = remove(n.right, value)
	case n.count > 1:
		n.count--
	case n.left == nil:
		return n.right, true
	case n.right == nil:
		return n.left, true
	default: // two children: replace with in-order successor
		var succ *node
		n.right, succ = removeMin(n.right)
		succ.left, succ.right = n.left, n.right
		n = succ
	}
	if !found {
		return n, false
	}
	return rebalance(n), true
}

func find(n *node, value int64) *node {
	for n != nil {
		switch {
		case value < n.value:
			n = n.left
		case value > n.value:
			n = n.right
		default:
			return n
		}
	}
	return nil
}

// Multiset is a concurrent multiset of int64 values with order statistics.
// The zero value is ready to use.
type Multiset struct {
	mu       sync.RWMutex
	root     *node
	distinct int
	closed   bool
}

// New returns an empty Multiset.
func New() *Multiset {
	return &Multiset{}
}

// Add inserts one occurrence of value. It fails with ErrClosed after Close.
func (m *Multiset) Add(value int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	var created bool
	m.root, created = insert(m.root, value)
	if created {
		m.distinct++
	}
	return nil
}

// Remove deletes one occurrence of value. It fails with ErrNotFound if the
// value is absent and with ErrClosed after Close.
func (m *Multiset) Remove(value int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	n := find(m.root, value)
	if n == nil {
		return fmt.Errorf("orderstatmultiset: remove %d: %w", value, ErrNotFound)
	}
	if n.count == 1 {
		m.distinct--
	}
	m.root, _ = remove(m.root, value)
	return nil
}

// Count returns how many times value occurs.
func (m *Multiset) Count(value int64) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if n := find(m.root, value); n != nil {
		return n.count
	}
	return 0
}

// Rank returns the number of elements strictly less than value.
func (m *Multiset) Rank(value int64) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rank := 0
	for n := m.root; n != nil; {
		switch {
		case value < n.value:
			n = n.left
		case value > n.value:
			rank += size(n.left) + n.count
			n = n.right
		default:
			return rank + size(n.left)
		}
	}
	return rank
}

// Kth returns the k-th smallest element (0-based); duplicates count
// individually. It fails with ErrOutOfRange if k is outside [0, Len()).
func (m *Multiset) Kth(k int) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if k < 0 || k >= size(m.root) {
		return 0, fmt.Errorf("orderstatmultiset: kth %d of %d: %w", k, size(m.root), ErrOutOfRange)
	}
	n := m.root
	for {
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
}

// Snapshot returns an independent sorted copy of all elements.
func (m *Multiset) Snapshot() []int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]int64, 0, size(m.root))
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

// Len returns the total number of elements, counting duplicates.
func (m *Multiset) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return size(m.root)
}

// DistinctLen returns the number of distinct values.
func (m *Multiset) DistinctLen() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.distinct
}

// Close rejects future Add and Remove calls. Queries remain available and
// Close may be called multiple times.
func (m *Multiset) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
}
