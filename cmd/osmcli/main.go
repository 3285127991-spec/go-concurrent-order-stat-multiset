// Command osmcli demonstrates the orderstatmultiset library: duplicates,
// Rank, Kth, removal, and queries after Close.
package main

import (
	"fmt"

	"orderstatmultiset"
)

func main() {
	m := orderstatmultiset.New()

	fmt.Println("== Add values (with duplicates): 5 2 8 2 5 5 1 ==")
	for _, v := range []int64{5, 2, 8, 2, 5, 5, 1} {
		if err := m.Add(v); err != nil {
			fmt.Println("add error:", err)
		}
	}
	fmt.Println("Snapshot:", m.Snapshot())
	fmt.Println("Len:", m.Len(), "DistinctLen:", m.DistinctLen())
	fmt.Println("Count(5):", m.Count(5), "Count(2):", m.Count(2), "Count(9):", m.Count(9))

	fmt.Println()
	fmt.Println("== Rank / Kth ==")
	for _, v := range []int64{0, 2, 5, 8, 9} {
		fmt.Printf("Rank(%d) = %d\n", v, m.Rank(v))
	}
	for k := 0; k < m.Len(); k++ {
		v, _ := m.Kth(k)
		fmt.Printf("Kth(%d) = %d\n", k, v)
	}

	fmt.Println()
	fmt.Println("== Remove ==")
	if err := m.Remove(5); err == nil {
		fmt.Println("removed one 5; Count(5):", m.Count(5), "Len:", m.Len())
	}
	if err := m.Remove(99); err != nil {
		fmt.Println("Remove(99):", err)
	}
	if _, err := m.Kth(m.Len()); err != nil {
		fmt.Println("Kth(Len):", err)
	}
	fmt.Println("Snapshot:", m.Snapshot())

	fmt.Println()
	fmt.Println("== Close, then keep querying ==")
	m.Close()
	m.Close() // idempotent
	if err := m.Add(10); err != nil {
		fmt.Println("Add(10):", err)
	}
	if err := m.Remove(2); err != nil {
		fmt.Println("Remove(2):", err)
	}
	fmt.Println("Snapshot:", m.Snapshot())
	fmt.Println("Len:", m.Len(), "DistinctLen:", m.DistinctLen())
	fmt.Println("Rank(5):", m.Rank(5))
	if v, err := m.Kth(0); err == nil {
		fmt.Println("Kth(0):", v)
	}
}
