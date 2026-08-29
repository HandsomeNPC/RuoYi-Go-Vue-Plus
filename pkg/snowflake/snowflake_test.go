package snowflake

import (
	"sync"
	"testing"
)

// TestNewRejectsOutOfRangeIDs 机器号越界应报错。
func TestNewRejectsOutOfRangeIDs(t *testing.T) {
	cases := []struct {
		name                   string
		workerID, datacenterID int64
	}{
		{"workerId 负数", -1, 0},
		{"workerId 超上界", maxWorkerID + 1, 0},
		{"datacenterId 负数", 0, -1},
		{"datacenterId 超上界", 0, maxDatacenterID + 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := New(c.workerID, c.datacenterID); err == nil {
				t.Errorf("New(%d, %d) 未报错，期望越界错误", c.workerID, c.datacenterID)
			}
		})
	}
}

// TestNewAcceptsBoundaryIDs 上下边界值应被接受。
func TestNewAcceptsBoundaryIDs(t *testing.T) {
	for _, c := range [][2]int64{{0, 0}, {maxWorkerID, maxDatacenterID}} {
		if _, err := New(c[0], c[1]); err != nil {
			t.Errorf("New(%d, %d) 报错: %v", c[0], c[1], err)
		}
	}
}

// TestNextIsMonotonicAndUnique 连续发号应严格递增且不重复。
// 数量取 10000 > 4096(单毫秒序列上限)，以覆盖序列号用尽后跨毫秒的分支。
func TestNextIsMonotonicAndUnique(t *testing.T) {
	g, err := New(1, 1)
	if err != nil {
		t.Fatalf("New 报错: %v", err)
	}

	const n = 10000
	seen := make(map[int64]struct{}, n)
	var prev int64
	for i := range n {
		id := g.Next()
		if id <= 0 {
			t.Fatalf("第 %d 个 ID = %d，应为正数", i, id)
		}
		if id <= prev {
			t.Fatalf("第 %d 个 ID = %d 未递增，前一个 = %d", i, id, prev)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("第 %d 个 ID = %d 重复", i, id)
		}
		seen[id] = struct{}{}
		prev = id
	}
}

// TestNextConcurrentUnique 并发发号不应撞号（验证锁有效）。
func TestNextConcurrentUnique(t *testing.T) {
	g, err := New(2, 3)
	if err != nil {
		t.Fatalf("New 报错: %v", err)
	}

	const goroutines, perGoroutine = 20, 500
	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		ids = make(map[int64]struct{}, goroutines*perGoroutine)
	)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := make([]int64, 0, perGoroutine)
			for range perGoroutine {
				local = append(local, g.Next())
			}
			mu.Lock()
			defer mu.Unlock()
			for _, id := range local {
				ids[id] = struct{}{}
			}
		}()
	}
	wg.Wait()

	if got, want := len(ids), goroutines*perGoroutine; got != want {
		t.Errorf("去重后 ID 数 = %d, want %d（有 %d 个撞号）", got, want, want-got)
	}
}

// TestNextEncodesMachineIDs 发出的 ID 应能还原出配置的机器号。
func TestNextEncodesMachineIDs(t *testing.T) {
	const workerID, datacenterID int64 = 7, 11
	g, err := New(workerID, datacenterID)
	if err != nil {
		t.Fatalf("New 报错: %v", err)
	}

	id := g.Next()
	if got := (id >> workerIDShift) & maxWorkerID; got != workerID {
		t.Errorf("ID 内 workerId = %d, want %d", got, workerID)
	}
	if got := (id >> datacenterIDShift) & maxDatacenterID; got != datacenterID {
		t.Errorf("ID 内 datacenterId = %d, want %d", got, datacenterID)
	}
}

// TestDifferentWorkersDoNotCollide 不同 workerId 在同一毫秒内不应发出相同 ID。
// 这是多进程共库的前提，故单独断言。
func TestDifferentWorkersDoNotCollide(t *testing.T) {
	g1, err := New(1, 0)
	if err != nil {
		t.Fatalf("New 报错: %v", err)
	}
	g2, err := New(2, 0)
	if err != nil {
		t.Fatalf("New 报错: %v", err)
	}

	ids := make(map[int64]struct{}, 2000)
	for range 1000 {
		for _, id := range []int64{g1.Next(), g2.Next()} {
			if _, dup := ids[id]; dup {
				t.Fatalf("不同 workerId 发出重复 ID %d", id)
			}
			ids[id] = struct{}{}
		}
	}
}

// TestNextPanicsWithoutInit 未 Init 就调包级 Next 应 panic。
func TestNextPanicsWithoutInit(t *testing.T) {
	mu.Lock()
	saved := defaultGenerator
	defaultGenerator = nil
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		defaultGenerator = saved
		mu.Unlock()
	})

	defer func() {
		if recover() == nil {
			t.Error("未 Init 时 Next() 未 panic")
		}
	}()
	Next()
}
