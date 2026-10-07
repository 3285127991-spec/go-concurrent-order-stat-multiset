# orderstatmultiset

基于 AVL 树实现的并发 Order-Statistic Multiset（Go 库 + 演示 CLI），纯标准库、零第三方依赖，Windows 本地直接运行。

## 特性

- 存储 `int64`，允许重复值；重复值只增加节点计数，不创建多余节点
- 每个节点维护 `count`（重复次数）与 `size`（子树元素总数），删除与旋转后正确修复
- `Add` / `Remove` / `Count` / `Rank` / `Kth` 均为 O(log n)，不复制、不排序全量数据
- `sync.RWMutex` 保护，多 goroutine 可安全并发读写
- `Close` 幂等；关闭后 `Add` / `Remove` 返回 `ErrClosed`，所有查询仍可用

## 复杂度

| 操作 | 复杂度 |
| --- | --- |
| `Add` / `Remove` / `Count` | O(log n) |
| `Rank` / `Kth` | O(log n) |
| `Len` / `DistinctLen` | O(1) |
| `Snapshot` | O(n) |

## API

```go
m := multiset.New()
err := m.Add(v)            // 插入一个 v；Close 后返回 ErrClosed
err  = m.Remove(v)         // 删除一个 v；不存在返回 ErrNotFound，Close 后返回 ErrClosed
n   := m.Count(v)          // v 的出现次数
r   := m.Rank(v)           // 严格小于 v 的元素总数
v, err = m.Kth(k)          // 第 k 小（0 起，重复按次数计）；越界返回 ErrOutOfRange
s   := m.Snapshot()        // 全部元素的升序独立副本
n   = m.Len()              // 元素总数（含重复）
n   = m.DistinctLen()      // 不同值的个数（节点数）
m.Close()                  // 幂等；关闭后拒绝写入，查询仍可用
```

## 运行

```sh
go run ./cmd/osms-cli      # 演示：重复值、Rank、Kth、删除、Close 后查询
go test -race ./...        # 8 个测试，含竞态检测
```

## 设计说明

- **自平衡 BST**：AVL 树，插入/删除沿路径回溯时通过 `recalc` 修复高度与 `size`，必要时做 LL/RR/LR/RL 旋转。
- **删除**：计数大于 1 时仅减计数；删最后一个时才移除节点。双子节点情形用中序后继拼接（`deleteMin`），随后重新平衡。
- **Rank/Kth**：沿树下行，利用左子树 `size` 与节点 `count` 累计/定位，O(log n)。
- **并发**：单把 `RWMutex`——写操作互斥，查询可并发；统计字段与树结构在同一把锁下更新，保证一致性。

明确不包含：持久化、范围查询、网络、GUI、数据库。
