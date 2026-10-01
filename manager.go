package reqdep

import "fmt"

// Manager 管理一个项目的需求依赖关系。
// 需求使用唯一标识，载入后需求集合不增删。
type Manager struct {
	reqs map[string]struct{}
	// edges 以 阻塞者 -> 被阻塞者集合 的形式保存当前依赖。
	edges map[string]map[string]struct{}
}

// NewManager 创建一个空的依赖管理器。
func NewManager() *Manager {
	return &Manager{
		reqs:  make(map[string]struct{}),
		edges: make(map[string]map[string]struct{}),
	}
}

// Load 载入需求集合及初始依赖。初始依赖保证无环。
// edges 中每个元素为 (blocker, blocked)，表示 blocker 阻塞 blocked。
// 重复调用将覆盖之前的状态。
func (m *Manager) Load(reqs []string, edges [][2]string) error {
	reqSet := make(map[string]struct{}, len(reqs))
	for _, r := range reqs {
		if _, dup := reqSet[r]; dup {
			return fmt.Errorf("reqdep: 需求标识重复: %q", r)
		}
		reqSet[r] = struct{}{}
	}
	edgeMap := make(map[string]map[string]struct{})
	for _, e := range edges {
		blocker, blocked := e[0], e[1]
		if _, ok := reqSet[blocker]; !ok {
			return fmt.Errorf("reqdep: 依赖引用了未载入的需求: %q", blocker)
		}
		if _, ok := reqSet[blocked]; !ok {
			return fmt.Errorf("reqdep: 依赖引用了未载入的需求: %q", blocked)
		}
		addEdge(edgeMap, blocker, blocked)
	}
	if hasCycle(edgeMap) {
		return fmt.Errorf("reqdep: 初始依赖存在环")
	}
	m.reqs = reqSet
	m.edges = edgeMap
	return nil
}

// Commit 提交一批依赖调整。
// 仅当全部调整应用后的关系图无环时整批生效并返回 true；
// 出现任何环（包括自环）则整批拒绝、状态完全不变并返回 false。
func (m *Manager) Commit(ops []Op) bool {
	next := cloneEdges(m.edges)
	for _, op := range ops {
		switch op.Type {
		case OpAdd:
			addEdge(next, op.Blocker, op.Blocked)
		case OpRemove:
			removeEdge(next, op.Blocker, op.Blocked)
		}
	}
	if hasCycle(next) {
		return false
	}
	m.edges = next
	return true
}

// BlockedBy 返回被 req 阻塞的全部直接及间接需求。
// 每项只出现一次，不包含 req 本身；返回顺序不确定。
// 每次调用都重新遍历并返回新切片，后续调整不会改写已返回的结果。
func (m *Manager) BlockedBy(req string) []string {
	visited := map[string]struct{}{req: {}}
	var result []string
	queue := []string{req}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for next := range m.edges[cur] {
			if _, seen := visited[next]; seen {
				continue
			}
			visited[next] = struct{}{}
			result = append(result, next)
			queue = append(queue, next)
		}
	}
	return result
}

func addEdge(edges map[string]map[string]struct{}, blocker, blocked string) {
	set, ok := edges[blocker]
	if !ok {
		set = make(map[string]struct{})
		edges[blocker] = set
	}
	set[blocked] = struct{}{}
}

func removeEdge(edges map[string]map[string]struct{}, blocker, blocked string) {
	set, ok := edges[blocker]
	if !ok {
		return
	}
	delete(set, blocked)
	if len(set) == 0 {
		delete(edges, blocker)
	}
}

func cloneEdges(edges map[string]map[string]struct{}) map[string]map[string]struct{} {
	cloned := make(map[string]map[string]struct{}, len(edges))
	for from, set := range edges {
		newSet := make(map[string]struct{}, len(set))
		for to := range set {
			newSet[to] = struct{}{}
		}
		cloned[from] = newSet
	}
	return cloned
}

// hasCycle 用 Kahn 拓扑排序判定有向图是否含环（含自环）。
func hasCycle(edges map[string]map[string]struct{}) bool {
	indegree := make(map[string]int)
	for from, set := range edges {
		if _, ok := indegree[from]; !ok {
			indegree[from] = 0
		}
		for to := range set {
			indegree[to]++
		}
	}
	var queue []string
	for node, deg := range indegree {
		if deg == 0 {
			queue = append(queue, node)
		}
	}
	removed := 0
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		removed++
		for next := range edges[cur] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	return removed < len(indegree)
}
