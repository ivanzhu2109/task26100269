package reqdep

import (
	"sort"
	"testing"
)

func sortedStrings(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

func assertSetEqual(t *testing.T, got []string, want ...string) {
	t.Helper()
	gotSorted := sortedStrings(got)
	wantSorted := sortedStrings(want)
	if len(gotSorted) != len(wantSorted) {
		t.Fatalf("结果长度不符: got %v, want %v", gotSorted, wantSorted)
	}
	for i := range gotSorted {
		if gotSorted[i] != wantSorted[i] {
			t.Fatalf("结果不符: got %v, want %v", gotSorted, wantSorted)
		}
	}
}

// 场景一：初始 A->B->C。
// 批次1（解除 B->C、建立 C->A）必须成功，查询 C 返回 A、B。
// 批次2（解除 A->B、建立 A->C）必须拒绝，查询 C 仍返回 A、B。
func TestScenarioChainReplan(t *testing.T) {
	m := NewManager()
	if err := m.Load([]string{"A", "B", "C"}, [][2]string{{"A", "B"}, {"B", "C"}}); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	assertSetEqual(t, m.BlockedBy("A"), "B", "C")

	// 若按顺序逐条应用，先建 C->A 会经过有环的中间状态；
	// 但最终图无环，整批必须成功。
	ok := m.Commit([]Op{
		{Type: OpAdd, Blocker: "C", Blocked: "A"},
		{Type: OpRemove, Blocker: "B", Blocked: "C"},
	})
	if !ok {
		t.Fatal("批次1 应被接受")
	}
	assertSetEqual(t, m.BlockedBy("C"), "A", "B")
	assertSetEqual(t, m.BlockedBy("A"), "B")
	assertSetEqual(t, m.BlockedBy("B"))

	// 解除 A->B 并建立 A->C：最终图 C->A->C 成环，整批拒绝。
	ok = m.Commit([]Op{
		{Type: OpRemove, Blocker: "A", Blocked: "B"},
		{Type: OpAdd, Blocker: "A", Blocked: "C"},
	})
	if ok {
		t.Fatal("批次2 应被拒绝")
	}
	// 拒绝后状态完全不变。
	assertSetEqual(t, m.BlockedBy("C"), "A", "B")
	assertSetEqual(t, m.BlockedBy("A"), "B")
	assertSetEqual(t, m.BlockedBy("B"))
}

// 场景二：A 分别阻塞 B、C，B、C 分别阻塞 D（菱形）。
// 解除 B->D 后 A 仍阻塞 D；再解除 C->D 后 D 才消失。
// 已返回的查询结果不被后续调整改写。
func TestScenarioDiamondMultiPath(t *testing.T) {
	m := NewManager()
	if err := m.Load([]string{"A", "B", "C", "D"}, [][2]string{
		{"A", "B"}, {"A", "C"}, {"B", "D"}, {"C", "D"},
	}); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	before := m.BlockedBy("A")
	assertSetEqual(t, before, "B", "C", "D")

	if !m.Commit([]Op{{Type: OpRemove, Blocker: "B", Blocked: "D"}}) {
		t.Fatal("解除 B->D 应成功")
	}
	// 仍可通过 A->C->D 阻塞 D。
	assertSetEqual(t, m.BlockedBy("A"), "B", "C", "D")

	if !m.Commit([]Op{{Type: OpRemove, Blocker: "C", Blocked: "D"}}) {
		t.Fatal("解除 C->D 应成功")
	}
	// 最后一条可达路径被解除，D 才消失。
	assertSetEqual(t, m.BlockedBy("A"), "B", "C")

	// 此前返回的结果保持原值。
	assertSetEqual(t, before, "B", "C", "D")
}

// 多条分别合法的新依赖合起来成环，必须整批拒绝。
func TestCommitRejectsCombinedCycle(t *testing.T) {
	m := NewManager()
	if err := m.Load([]string{"A", "B", "C"}, [][2]string{{"A", "B"}}); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	// 单独看 B->C、C->A 各自都合法，合起来形成 A->B->C->A 环。
	ok := m.Commit([]Op{
		{Type: OpAdd, Blocker: "B", Blocked: "C"},
		{Type: OpAdd, Blocker: "C", Blocked: "A"},
	})
	if ok {
		t.Fatal("合起来成环的批次应被拒绝")
	}
	// 状态不变：两条新依赖都未生效。
	assertSetEqual(t, m.BlockedBy("A"), "B")
	assertSetEqual(t, m.BlockedBy("B"))
	assertSetEqual(t, m.BlockedBy("C"))
}

// 自环（自己阻塞自己）必须被拒绝。
func TestCommitRejectsSelfLoop(t *testing.T) {
	m := NewManager()
	if err := m.Load([]string{"A"}, nil); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if m.Commit([]Op{{Type: OpAdd, Blocker: "A", Blocked: "A"}}) {
		t.Fatal("自环批次应被拒绝")
	}
	assertSetEqual(t, m.BlockedBy("A"))
}

// 间接自环：A->B 已存在，再建 B->A 形成二元环。
func TestCommitRejectsIndirectCycle(t *testing.T) {
	m := NewManager()
	if err := m.Load([]string{"A", "B"}, [][2]string{{"A", "B"}}); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if m.Commit([]Op{{Type: OpAdd, Blocker: "B", Blocked: "A"}}) {
		t.Fatal("形成二元环的批次应被拒绝")
	}
	assertSetEqual(t, m.BlockedBy("A"), "B")
	assertSetEqual(t, m.BlockedBy("B"))
}

// 空批次应成功且不改变状态。
func TestCommitEmptyBatch(t *testing.T) {
	m := NewManager()
	if err := m.Load([]string{"A", "B"}, [][2]string{{"A", "B"}}); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if !m.Commit(nil) {
		t.Fatal("空批次应被接受")
	}
	assertSetEqual(t, m.BlockedBy("A"), "B")
}

// 多轮调整：同一批内先解除旧依赖再建立新依赖，逐次验证。
func TestMultiRoundAdjustments(t *testing.T) {
	m := NewManager()
	if err := m.Load([]string{"A", "B", "C", "D"}, [][2]string{
		{"A", "B"}, {"B", "C"}, {"C", "D"},
	}); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	assertSetEqual(t, m.BlockedBy("A"), "B", "C", "D")

	// 第一轮：解除 B->C，建立 A->C。最终 A->B、A->C、C->D。
	if !m.Commit([]Op{
		{Type: OpRemove, Blocker: "B", Blocked: "C"},
		{Type: OpAdd, Blocker: "A", Blocked: "C"},
	}) {
		t.Fatal("第一轮应成功")
	}
	assertSetEqual(t, m.BlockedBy("A"), "B", "C", "D")
	assertSetEqual(t, m.BlockedBy("B"))
	assertSetEqual(t, m.BlockedBy("C"), "D")

	// 第二轮：建立 D->A 会形成 A->C->D->A 环，拒绝。
	if m.Commit([]Op{{Type: OpAdd, Blocker: "D", Blocked: "A"}}) {
		t.Fatal("第二轮应被拒绝")
	}
	assertSetEqual(t, m.BlockedBy("A"), "B", "C", "D")
	assertSetEqual(t, m.BlockedBy("D"))

	// 第三轮：解除 C->D 后 D->A 合法。
	if !m.Commit([]Op{
		{Type: OpRemove, Blocker: "C", Blocked: "D"},
		{Type: OpAdd, Blocker: "D", Blocked: "A"},
	}) {
		t.Fatal("第三轮应成功")
	}
	assertSetEqual(t, m.BlockedBy("D"), "A", "B", "C")
	assertSetEqual(t, m.BlockedBy("A"), "B", "C")
	assertSetEqual(t, m.BlockedBy("C"))
}

// Load 的入参校验：重复需求、未知需求、初始环。
func TestLoadValidation(t *testing.T) {
	m := NewManager()
	if err := m.Load([]string{"A", "A"}, nil); err == nil {
		t.Fatal("重复需求应报错")
	}
	if err := m.Load([]string{"A"}, [][2]string{{"A", "B"}}); err == nil {
		t.Fatal("引用未载入需求应报错")
	}
	if err := m.Load([]string{"A", "B"}, [][2]string{{"A", "B"}, {"B", "A"}}); err == nil {
		t.Fatal("初始依赖成环应报错")
	}
}

// 查询未知需求返回空结果。
func TestBlockedByUnknownReq(t *testing.T) {
	m := NewManager()
	if err := m.Load([]string{"A"}, nil); err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	assertSetEqual(t, m.BlockedBy("Z"))
}
