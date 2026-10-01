// Package reqdep 提供进程内存中的需求依赖管理：
// 载入需求与初始依赖、提交一批依赖调整、查询下游影响。
package reqdep

// Edge 表示一条依赖关系：Blocker 阻塞 Blocked（即 Blocked 依赖 Blocker）。
type Edge struct {
	Blocker string
	Blocked string
}

// OpType 表示一批调整中单个操作的类型。
type OpType int

const (
	// OpAdd 建立一条依赖关系。
	OpAdd OpType = iota
	// OpRemove 解除一条依赖关系。
	OpRemove
)

// Adjustment 表示一次依赖调整操作。
type Adjustment struct {
	Op   OpType
	Edge Edge
}
