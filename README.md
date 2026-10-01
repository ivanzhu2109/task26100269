# reqdep — 需求依赖管理库

进程内存中的需求依赖管理库，由 SprintReq 上层直接调用，无接口层、无外部服务。
只处理一个项目，需求使用唯一字符串标识，载入后需求不增删。

## 能力

- `Load`：载入需求及初始依赖（初始依赖保证无环）。
- `Submit`：提交一批依赖调整（建立 / 解除），整批原子生效。
- `QueryBlocked`：沿阻塞方向查询全部直接及间接受阻需求。

## 语义约定

- `Edge{Blocker, Blocked}` 表示 Blocker 阻塞 Blocked（Blocked 依赖 Blocker）。
- `Submit` 只依据全部调整后的最终关系判环：最终无环则整批生效返回 `true`；
  出现任何环（含自环）则整批拒绝返回 `false`，原有依赖完全不变。
  中间状态有环不影响判定。
- `QueryBlocked` 返回去重后的受阻需求（字典序），不包含起点；
  每次调用返回新切片，已返回结果不受后续调整影响。
- 一项需求可经多条路径阻塞同一目标，只有最后一条可达路径被解除，
  目标才从查询结果中消失（由图可达性自然保证）。

## 使用示例

```go
g := reqdep.New()
g.Load(
    []string{"A", "B", "C"},
    []reqdep.Edge{
        {Blocker: "A", Blocked: "B"},
        {Blocker: "B", Blocked: "C"},
    },
)

// 一批内同时解除旧依赖、建立新依赖
ok := g.Submit([]reqdep.Adjustment{
    {Op: reqdep.OpRemove, Edge: reqdep.Edge{Blocker: "B", Blocked: "C"}},
    {Op: reqdep.OpAdd,    Edge: reqdep.Edge{Blocker: "C", Blocked: "A"}},
})
// ok == true，最终关系 A->B、C->A 无环

blocked := g.QueryBlocked("C") // ["A", "B"]
```

## 测试

```sh
go test ./... -v
```

覆盖小图多轮调整：提交判定、整批拒绝后的状态保持、多路径影响的逐步消失、
已返回查询结果的快照隔离、自环拒绝、中间态有环但最终合法的批次。
