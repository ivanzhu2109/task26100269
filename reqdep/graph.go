package reqdep

import "sort"

// Graph 保存一个项目的需求及其依赖关系，数据仅在进程内存中。
// edges 的键为阻塞方，值为被其阻塞的需求集合。
type Graph struct {
	reqs  map[string]bool
	edges map[string]map[string]bool
}

// New 创建一个空的依赖图。
func New() *Graph {
	return &Graph{
		reqs:  make(map[string]bool),
		edges: make(map[string]map[string]bool),
	}
}

// Load 载入需求及初始依赖。初始依赖保证无环，载入后需求不增删。
func (g *Graph) Load(reqs []string, deps []Edge) {
	for _, r := range reqs {
		g.reqs[r] = true
	}
	for _, e := range deps {
		g.addEdge(e)
	}
}

// addEdge 建立 Blocker -> Blocked 的边，调用方保证关系原先不存在。
func (g *Graph) addEdge(e Edge) {
	if g.edges[e.Blocker] == nil {
		g.edges[e.Blocker] = make(map[string]bool)
	}
	g.edges[e.Blocker][e.Blocked] = true
}

// removeEdge 解除 Blocker -> Blocked 的边，调用方保证关系原先存在。
func (g *Graph) removeEdge(e Edge) {
	delete(g.edges[e.Blocker], e.Blocked)
}

// QueryBlocked 沿阻塞方向返回全部直接及间接受阻需求，
// 每项只出现一次，不包含起点。返回结果不受后续调整影响。
func (g *Graph) QueryBlocked(req string) []string {
	visited := map[string]bool{req: true}
	queue := []string{req}
	var result []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for next := range g.edges[cur] {
			if visited[next] {
				continue
			}
			visited[next] = true
			result = append(result, next)
			queue = append(queue, next)
		}
	}
	sort.Strings(result)
	return result
}

// Submit 提交一批依赖调整。全部调整后关系无环则整批生效并返回 true；
// 出现任何环（包括自环）则整批拒绝，原有依赖保持不变，返回 false。
func (g *Graph) Submit(batch []Adjustment) bool {
	edges := make(map[string]map[string]bool, len(g.edges))
	for from, tos := range g.edges {
		copied := make(map[string]bool, len(tos))
		for to := range tos {
			copied[to] = true
		}
		edges[from] = copied
	}
	for _, adj := range batch {
		switch adj.Op {
		case OpAdd:
			if edges[adj.Edge.Blocker] == nil {
				edges[adj.Edge.Blocker] = make(map[string]bool)
			}
			edges[adj.Edge.Blocker][adj.Edge.Blocked] = true
		case OpRemove:
			delete(edges[adj.Edge.Blocker], adj.Edge.Blocked)
		}
	}
	if hasCycle(edges) {
		return false
	}
	g.edges = edges
	return true
}

// hasCycle 用 Kahn 算法判断图中是否存在环，自环也会被判定为环。
func hasCycle(edges map[string]map[string]bool) bool {
	indegree := make(map[string]int)
	for from, tos := range edges {
		if _, ok := indegree[from]; !ok {
			indegree[from] = 0
		}
		for to := range tos {
			indegree[to]++
		}
	}
	var queue []string
	for node, deg := range indegree {
		if deg == 0 {
			queue = append(queue, node)
		}
	}
	visited := 0
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		visited++
		for next := range edges[cur] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	return visited < len(indegree)
}
