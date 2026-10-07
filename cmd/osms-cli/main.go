// Command osms-cli demonstrates the multiset package: duplicates, Rank,
// Kth, removal and queries after Close.
package main

import (
	"fmt"

	"orderstatmultiset/multiset"
)

func main() {
	m := multiset.New()

	for _, v := range []int64{5, 1, 9, 1, 5, 3, 9, 9, 7} {
		if err := m.Add(v); err != nil {
			fmt.Println("add error:", err)
		}
	}
	fmt.Println("added:      5 1 9 1 5 3 9 9 7")
	fmt.Println("snapshot:  ", m.Snapshot())
	fmt.Println("len:       ", m.Len())
	fmt.Println("distinct:  ", m.DistinctLen())
	fmt.Println("count(9):  ", m.Count(9))
	fmt.Println("rank(5):   ", m.Rank(5), "(elements < 5)")
	fmt.Println("rank(6):   ", m.Rank(6), "(elements < 6)")

	for _, k := range []int{0, 4, m.Len() - 1} {
		if v, err := m.Kth(k); err == nil {
			fmt.Printf("kth(%d):     %d\n", k, v)
		}
	}

	mustRemove(m, 9) // one of three 9s: count drops, node stays
	fmt.Println("after remove(9):  count(9) =", m.Count(9), " len =", m.Len(), " distinct =", m.DistinctLen())
	mustRemove(m, 7) // last 7: node removed
	fmt.Println("after remove(7):  count(7) =", m.Count(7), " len =", m.Len(), " distinct =", m.DistinctLen())
	fmt.Println("snapshot:        ", m.Snapshot())

	fmt.Println("remove(42):      ", m.Remove(42))
	if _, err := m.Kth(m.Len()); err != nil {
		fmt.Println("kth(len):        ", err)
	}

	m.Close()
	m.Close() // idempotent
	fmt.Println("closed (twice)")
	fmt.Println("add(0):          ", m.Add(0))
	fmt.Println("remove(1):       ", m.Remove(1))
	fmt.Println("queries still work after close:")
	fmt.Println("  snapshot:", m.Snapshot())
	fmt.Println("  rank(9): ", m.Rank(9))
	if v, err := m.Kth(0); err == nil {
		fmt.Println("  kth(0):  ", v)
	}
	fmt.Println("  len/distinct:", m.Len(), "/", m.DistinctLen())
}

func mustRemove(m *multiset.Multiset, v int64) {
	if err := m.Remove(v); err != nil {
		fmt.Println("remove error:", err)
	}
}
