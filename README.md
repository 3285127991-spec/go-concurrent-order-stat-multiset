# orderstatmultiset

A concurrent order-statistic multiset for Go, implemented from scratch as an
AVL tree. Each node caches a duplicate count and its subtree size, so all
core operations run in O(log n) — Rank and Kth never copy or re-sort data.

No third-party dependencies. Library + small demo CLI + tests.

## Features

- Stores `int64` values with duplicates (one node per distinct value).
- `Add` / `Remove` / `Count` / `Rank` / `Kth` in O(log n).
- `Rank(v)` = number of elements strictly less than `v`.
- `Kth(k)` = k-th smallest element, 0-based, duplicates counted.
- `Snapshot()` = independent sorted copy of all elements.
- `Len()` counts duplicates; `DistinctLen()` counts distinct values.
- Safe for concurrent use (`sync.RWMutex`); `go test -race` clean.
- `Close()` rejects further writes; queries keep working; idempotent.

## Usage

```go
m := orderstatmultiset.New()

m.Add(5)          // duplicates allowed
m.Add(5)
m.Add(2)

m.Count(5)        // 2
m.Rank(5)         // 1  (only 2 is smaller)
m.Kth(0)          // 2, nil
m.Kth(2)          // 5, nil
m.Len()           // 3
m.DistinctLen()   // 2
m.Snapshot()      // [2 5 5]

m.Remove(5)       // removes one occurrence
m.Remove(9)       // error: ErrNotFound
m.Kth(10)         // error: ErrOutOfRange

m.Close()         // further Add/Remove return ErrClosed; queries still work
```

Errors are sentinel values (`ErrNotFound`, `ErrOutOfRange`, `ErrClosed`);
match them with `errors.Is`.

## Run

```sh
go test -race ./...   # run tests with the race detector
go run ./cmd/osmcli   # demo: duplicates, Rank, Kth, Remove, Close
```

## Layout

- `multiset.go` — AVL tree + public `Multiset` API.
- `multiset_test.go` — tests incl. AVL invariant checks and concurrency.
- `cmd/osmcli/main.go` — demo CLI.
