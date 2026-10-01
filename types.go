// Package reqdep 提供单项目的需求依赖（阻塞）关系管理。
// 数据保存在进程内存中，由 SprintReq 上层直接调用，无接口层。
package reqdep

// OpType 表示一批调整中的单个操作类型。
type OpType int

const (
	// OpAdd 建立依赖：Blocker 阻塞 Blocked（Blocked 依赖 Blocker）。
	OpAdd OpType = iota
	// OpRemove 解除已存在的依赖关系。
	OpRemove
)

// Op 表示一次依赖调整操作。
// 语义：Blocker 阻塞 Blocked，即 Blocked 依赖 Blocker。
type Op struct {
	Type    OpType
	Blocker string
	Blocked string
}
