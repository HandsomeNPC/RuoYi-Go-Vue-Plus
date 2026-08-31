package tree

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// dept 测试用业务对象。
type dept struct {
	id, pid  int64
	name     string
	order    int
	disabled bool
}

func parseDept(d dept, n *Tree[int64]) {
	n.ID, n.ParentID, n.Name, n.Weight = d.id, d.pid, d.name, d.order
	n.SetExtra("disabled", d.disabled)
}

// TestMarshalJSONMatchesHutool 断言序列化结果与 hutool 5.8.28 实跑输出一致。
// 期望值取自 TreeUtil.build(list, 0L, DEFAULT_CONFIG.setNameKey("label"), parser)。
func TestMarshalJSONMatchesHutool(t *testing.T) {
	list := []dept{
		{1, 0, "HQ", 1, false},
		{2, 1, "RD", 2, true},
		{3, 1, "Sales", 1, false},
	}
	got, err := json.Marshal(Build(list, int64(0), parseDept))
	if err != nil {
		t.Fatal(err)
	}

	want := `[{"id":1,"parentId":0,"label":"HQ","weight":1,"disabled":false,` +
		`"children":[{"id":3,"parentId":1,"label":"Sales","weight":1,"disabled":false},` +
		`{"id":2,"parentId":1,"label":"RD","weight":2,"disabled":true}]}]`
	if string(got) != want {
		t.Errorf("JSON 与 hutool 不一致\n got: %s\nwant: %s", got, want)
	}
}

// TestLeafOmitsChildrenKey 叶子节点不能出现 children 键：前端靠键的有无判叶子。
func TestLeafOmitsChildrenKey(t *testing.T) {
	got, err := json.Marshal(Build([]dept{{1, 0, "leaf", 1, false}}, int64(0), parseDept))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "children") {
		t.Errorf("叶子节点不应输出 children 键: %s", got)
	}
}

// TestExtraKeysSorted Extra 按字典序输出，保证同一份数据每次序列化结果稳定。
func TestExtraKeysSorted(t *testing.T) {
	n := &Tree[int64]{ID: 1, Name: "x"}
	n.SetExtra("zeta", 1).SetExtra("alpha", 2).SetExtra("mid", 3)

	// 多跑几次：若依赖 map 迭代序，这里会随机失败。
	want := `{"id":1,"parentId":0,"label":"x","weight":0,"alpha":2,"mid":3,"zeta":1}`
	for i := 0; i < 20; i++ {
		got, err := json.Marshal(n)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Fatalf("第 %d 次序列化键序不稳定\n got: %s\nwant: %s", i, got, want)
		}
	}
}

// TestSortByWeight 同层按 weight 升序，根层级同样参与排序。
func TestSortByWeight(t *testing.T) {
	list := []dept{{10, 0, "B-root", 5, false}, {11, 0, "A-root", 2, false}}
	roots := Build(list, int64(0), parseDept)
	if len(roots) != 2 || roots[0].ID != 11 || roots[1].ID != 10 {
		t.Errorf("根节点未按 weight 排序: %v", ids(roots))
	}
}

// TestSortStable weight 相同时保持插入序，对齐 hutool 基于 LinkedHashMap 的稳定排序。
//
// 规模和 weight 分布是刻意选的：Go 的 pdqsort 对小切片走插入排序，全等元素也不会乱序，
// 用三五个节点测不出稳定性差异。要让不稳定排序真的搬动元素，需要 16+ 个节点、
// 且重复 weight 穿插在不同 weight 之间（已验证 SortFunc 在此规模下会打乱组内顺序）。
func TestSortStable(t *testing.T) {
	const size = 24
	list := make([]dept, 0, size)
	for i := 0; i < size; i++ {
		list = append(list, dept{id: int64(i), pid: 0, order: i % 3})
	}

	roots := Build(list, int64(0), parseDept)
	if len(roots) != size {
		t.Fatalf("根节点数 = %d, 期望 %d", len(roots), size)
	}
	// 同 weight 分组内，id 必须仍按插入序递增。
	for i := 1; i < len(roots); i++ {
		prev, cur := roots[i-1], roots[i]
		if prev.Weight == cur.Weight && prev.ID > cur.ID {
			t.Fatalf("weight=%d 组内顺序被打乱: id %d 排在 %d 之前",
				cur.Weight, prev.ID, cur.ID)
		}
	}
}

// TestBuildOrphanDropped 父节点不在集合内的子树不应冒到顶层。
func TestBuildOrphanDropped(t *testing.T) {
	list := []dept{{1, 0, "root", 1, false}, {100, 1, "child", 1, false}, {200, 77, "orphan", 1, false}}
	roots := Build(list, int64(0), parseDept)
	if len(roots) != 1 || roots[0].ID != 1 {
		t.Fatalf("顶层节点 = %v, 期望仅 id=1", ids(roots))
	}
	if len(roots[0].Children) != 1 || roots[0].Children[0].ID != 100 {
		t.Errorf("子节点未正确挂接: %v", ids(roots[0].Children))
	}
}

// TestBuildEmpty 空输入返回空切片而非 nil，保证序列化成 [] 而非 null。
func TestBuildEmpty(t *testing.T) {
	got := Build(nil, int64(0), parseDept)
	if got == nil || len(got) != 0 {
		t.Fatalf("Build(nil) = %v, 期望空切片", got)
	}
	if b, _ := json.Marshal(got); string(b) != "[]" {
		t.Errorf("空树序列化 = %s, 期望 []", b)
	}
}

// TestBuildMultiRoot 悬空父级(祖先被过滤掉)各自成根。
func TestBuildMultiRoot(t *testing.T) {
	// 父级 100、200 都不在集合内，应识别出两个根。
	list := []dept{
		{1, 100, "a", 1, false},
		{2, 200, "b", 1, false},
		{3, 1, "a-child", 1, false},
	}
	roots := BuildMultiRoot(list,
		func(d dept) int64 { return d.id },
		func(d dept) int64 { return d.pid },
		parseDept)

	if got, want := ids(roots), []int64{1, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("多根 = %v, 期望 %v", got, want)
	}
	if len(roots[0].Children) != 1 || roots[0].Children[0].ID != 3 {
		t.Errorf("根 1 的子节点未挂接: %v", ids(roots[0].Children))
	}
}

// TestBuildMultiRootOrderStable 多根顺序须按入参首次出现序，不能随 map 迭代抖动。
func TestBuildMultiRootOrderStable(t *testing.T) {
	list := []dept{{1, 900, "a", 1, false}, {2, 800, "b", 1, false}, {3, 700, "c", 1, false}}
	want := []int64{1, 2, 3}
	for i := 0; i < 20; i++ {
		got := ids(BuildMultiRoot(list,
			func(d dept) int64 { return d.id },
			func(d dept) int64 { return d.pid },
			parseDept))
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("第 %d 次多根顺序 = %v, 期望 %v", i, got, want)
		}
	}
}

// menu 测试 BuildInPlace 用的自引用类型。
type menu struct {
	id, pid  int64
	order    int
	children []*menu
}

// TestBuildInPlaceKeepsInputOrder BuildInPlace 不排序，完全保留入参顺序。
func TestBuildInPlaceKeepsInputOrder(t *testing.T) {
	// order 故意逆序：若误加了排序，结果会变成 30/20/10。
	items := []*menu{{id: 1, pid: 0, order: 30}, {id: 2, pid: 0, order: 20}, {id: 3, pid: 0, order: 10}}
	roots := BuildInPlace(items, int64(0),
		func(m *menu) int64 { return m.id },
		func(m *menu) int64 { return m.pid },
		func(m *menu, c []*menu) { m.children = c })

	for i, want := range []int64{1, 2, 3} {
		if roots[i].id != want {
			t.Fatalf("第 %d 个根 = %d, 期望 %d(应保留入参顺序,不排序)", i, roots[i].id, want)
		}
	}
}

// TestBuildInPlaceOrphanDropped 就地构树同样不让孤儿子树冒顶。
func TestBuildInPlaceOrphanDropped(t *testing.T) {
	items := []*menu{{id: 1, pid: 0}, {id: 100, pid: 1}, {id: 200, pid: 99}}
	roots := BuildInPlace(items, int64(0),
		func(m *menu) int64 { return m.id },
		func(m *menu) int64 { return m.pid },
		func(m *menu, c []*menu) { m.children = c })

	if len(roots) != 1 || roots[0].id != 1 {
		t.Fatalf("顶层 = %d 个, 期望仅 id=1", len(roots))
	}
	if len(roots[0].children) != 1 || roots[0].children[0].id != 100 {
		t.Errorf("子节点未挂接: %v", roots[0].children)
	}
}

// TestBuildInPlaceEmpty 空输入返回空切片。
func TestBuildInPlaceEmpty(t *testing.T) {
	got := BuildInPlace(nil, int64(0),
		func(m *menu) int64 { return m.id },
		func(m *menu) int64 { return m.pid },
		func(m *menu, c []*menu) { m.children = c })
	if got == nil || len(got) != 0 {
		t.Errorf("BuildInPlace(nil) = %v, 期望空切片", got)
	}
}

// ids 提取节点 ID，便于断言顺序。
func ids[K comparable](nodes []*Tree[K]) []K {
	out := make([]K, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.ID)
	}
	return out
}
