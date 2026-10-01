package reqdep

import (
	"reflect"
	"testing"
)

func add(blocker, blocked string) Adjustment {
	return Adjustment{Op: OpAdd, Edge: Edge{Blocker: blocker, Blocked: blocked}}
}

func remove(blocker, blocked string) Adjustment {
	return Adjustment{Op: OpRemove, Edge: Edge{Blocker: blocker, Blocked: blocked}}
}

// 场景一：初始 A->B->C。
// 第一批解除 B->C、建立 C->A 必须成功，查询 C 返回 [A B]。
// 第二批解除 A->B、建立 A->C 必须拒绝，查询 C 仍返回 [A B]。
func TestBatchAtomicityAcrossRounds(t *testing.T) {
	g := New()
	g.Load([]string{"A", "B", "C"}, []Edge{
		{Blocker: "A", Blocked: "B"},
		{Blocker: "B", Blocked: "C"},
	})

	if got := g.QueryBlocked("A"); !reflect.DeepEqual(got, []string{"B", "C"}) {
		t.Fatalf("初始查询 A = %v, 期望 [B C]", got)
	}
	if got := g.QueryBlocked("C"); len(got) != 0 {
		t.Fatalf("初始查询 C = %v, 期望为空", got)
	}

	if !g.Submit([]Adjustment{remove("B", "C"), add("C", "A")}) {
		t.Fatal("第一批（解除 B->C，建立 C->A）应成功")
	}
	if got := g.QueryBlocked("C"); !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Fatalf("第一批后查询 C = %v, 期望 [A B]", got)
	}

	if g.Submit([]Adjustment{remove("A", "B"), add("A", "C")}) {
		t.Fatal("第二批（解除 A->B，建立 A->C）应拒绝")
	}
	if got := g.QueryBlocked("C"); !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Fatalf("拒绝后查询 C = %v, 期望仍为 [A B]", got)
	}
	if got := g.QueryBlocked("A"); !reflect.DeepEqual(got, []string{"B"}) {
		t.Fatalf("拒绝后查询 A = %v, 期望仍为 [B]", got)
	}
}

// 场景二：A->B、A->C、B->D、C->D。
// 解除 B->D 后查询 A 仍含 D；再解除 C->D 后才不含 D。
// 已返回的查询结果不能被后续调整改写。
func TestMultiplePathsAndSnapshotIsolation(t *testing.T) {
	g := New()
	g.Load([]string{"A", "B", "C", "D"}, []Edge{
		{Blocker: "A", Blocked: "B"},
		{Blocker: "A", Blocked: "C"},
		{Blocker: "B", Blocked: "D"},
		{Blocker: "C", Blocked: "D"},
	})

	before := g.QueryBlocked("A")
	if !reflect.DeepEqual(before, []string{"B", "C", "D"}) {
		t.Fatalf("初始查询 A = %v, 期望 [B C D]", before)
	}

	if !g.Submit([]Adjustment{remove("B", "D")}) {
		t.Fatal("解除 B->D 应成功")
	}
	if got := g.QueryBlocked("A"); !reflect.DeepEqual(got, []string{"B", "C", "D"}) {
		t.Fatalf("解除 B->D 后查询 A = %v, 期望仍含 D: [B C D]", got)
	}

	if !g.Submit([]Adjustment{remove("C", "D")}) {
		t.Fatal("解除 C->D 应成功")
	}
	if got := g.QueryBlocked("A"); !reflect.DeepEqual(got, []string{"B", "C"}) {
		t.Fatalf("解除 C->D 后查询 A = %v, 期望 [B C]", got)
	}

	if !reflect.DeepEqual(before, []string{"B", "C", "D"}) {
		t.Fatalf("此前返回的查询结果被改写为 %v, 期望保持 [B C D]", before)
	}
}

// 整批拒绝：多条分别合法的新依赖合起来成环时必须整批拒绝。
func TestBatchCycleAcrossMultipleAdds(t *testing.T) {
	g := New()
	g.Load([]string{"A", "B", "C"}, []Edge{
		{Blocker: "A", Blocked: "B"},
	})

	if g.Submit([]Adjustment{add("B", "C"), add("C", "A")}) {
		t.Fatal("B->C 与 C->A 合起来成环，应整批拒绝")
	}
	if got := g.QueryBlocked("A"); !reflect.DeepEqual(got, []string{"B"}) {
		t.Fatalf("拒绝后查询 A = %v, 期望仍为 [B]", got)
	}
	if got := g.QueryBlocked("B"); len(got) != 0 {
		t.Fatalf("拒绝后查询 B = %v, 期望为空", got)
	}
}

// 自环也是环，必须拒绝且状态不变。
func TestSelfLoopRejected(t *testing.T) {
	g := New()
	g.Load([]string{"A", "B"}, []Edge{
		{Blocker: "A", Blocked: "B"},
	})

	if g.Submit([]Adjustment{add("A", "A")}) {
		t.Fatal("建立 A->A 自环应拒绝")
	}
	if got := g.QueryBlocked("A"); !reflect.DeepEqual(got, []string{"B"}) {
		t.Fatalf("拒绝后查询 A = %v, 期望仍为 [B]", got)
	}
}

// 中间状态有环但最终合法的批次必须成功。
func TestIntermediateCycleAllowed(t *testing.T) {
	g := New()
	g.Load([]string{"A", "B"}, []Edge{
		{Blocker: "A", Blocked: "B"},
	})

	// 若按顺序逐条判定，先建立 B->A 会形成环；但整批解除 A->B 后合法。
	if !g.Submit([]Adjustment{add("B", "A"), remove("A", "B")}) {
		t.Fatal("最终无环的批次应成功，不能因中间状态有环而拒绝")
	}
	if got := g.QueryBlocked("B"); !reflect.DeepEqual(got, []string{"A"}) {
		t.Fatalf("提交后查询 B = %v, 期望 [A]", got)
	}
	if got := g.QueryBlocked("A"); len(got) != 0 {
		t.Fatalf("提交后查询 A = %v, 期望为空", got)
	}
}

// 空批次应成功且不改变状态。
func TestEmptyBatch(t *testing.T) {
	g := New()
	g.Load([]string{"A"}, nil)

	if !g.Submit(nil) {
		t.Fatal("空批次应成功")
	}
	if got := g.QueryBlocked("A"); len(got) != 0 {
		t.Fatalf("查询 A = %v, 期望为空", got)
	}
}
