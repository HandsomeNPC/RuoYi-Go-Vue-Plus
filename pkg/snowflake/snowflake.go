// Package snowflake 雪花算法 ID 生成器，对照 Java MyBatis-Plus 的 ASSIGN_ID 主键策略。
//
// RuoYi 各业务表主键都是 `bigint not null` 且不带 auto_increment，插入时必须由应用层
// 发号。Java 侧由 MyBatis-Plus 自动填充，Go 侧 GORM 没有等价机制，故在 repository
// 插入前显式调用 Next()。
//
// 初始化对照 redis.Init / captcha.Init：snowflake.Init() 无参，自读
// config.Get().Snowflake；业务侧用包级 snowflake.Next()。
//
// 64 位布局与 Twitter 原版一致：
//
//	1 位符号位(恒 0) | 41 位毫秒时间戳 | 5 位数据中心号 | 5 位工作机器号 | 12 位序列号
//
// 同一毫秒内最多发 4096 个号，超出则自旋等到下一毫秒。**多进程部署时每个进程必须
// 配不同的 workerId**，否则会撞号，见 configs/<module>.yaml。
package snowflake

import (
	"fmt"
	"log"
	"sync"
	"time"

	"ruoyi-go-vue-plus/pkg/config"
)

// 各段位宽。
const (
	workerIDBits     = 5
	datacenterIDBits = 5
	sequenceBits     = 12
)

// 各段最大值与位移量。
const (
	maxWorkerID     int64 = -1 ^ (-1 << workerIDBits)     // 31
	maxDatacenterID int64 = -1 ^ (-1 << datacenterIDBits) // 31
	sequenceMask    int64 = -1 ^ (-1 << sequenceBits)     // 4095

	workerIDShift     = sequenceBits                                   // 12
	datacenterIDShift = sequenceBits + workerIDBits                    // 17
	timestampShift    = sequenceBits + workerIDBits + datacenterIDBits // 22
)

// epoch 起始时间戳（毫秒），2020-01-01 00:00:00 UTC。
// 41 位毫秒从此刻起可用约 69 年。改这个值会让新旧 ID 大小关系错乱，上线后不可再动。
const epoch int64 = 1577836800000

// Generator 雪花 ID 生成器，并发安全。
type Generator struct {
	mu            sync.Mutex
	workerID      int64
	datacenterID  int64
	sequence      int64
	lastTimestamp int64
}

// New 构造生成器，机器号越界时返回错误。
func New(workerID, datacenterID int64) (*Generator, error) {
	if workerID < 0 || workerID > maxWorkerID {
		return nil, fmt.Errorf("snowflake: workerId %d 越界，须在 0-%d 之间", workerID, maxWorkerID)
	}
	if datacenterID < 0 || datacenterID > maxDatacenterID {
		return nil, fmt.Errorf("snowflake: datacenterId %d 越界，须在 0-%d 之间", datacenterID, maxDatacenterID)
	}
	return &Generator{
		workerID:      workerID,
		datacenterID:  datacenterID,
		lastTimestamp: -1,
	}, nil
}

// Next 返回下一个 ID。同毫秒内序列号用尽会自旋到下一毫秒。
func (g *Generator) Next() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := currentMillis()
	switch {
	case now == g.lastTimestamp:
		// 同一毫秒内递增序列号，用尽(4096 个)则等到下一毫秒。
		g.sequence = (g.sequence + 1) & sequenceMask
		if g.sequence == 0 {
			now = g.waitNextMillis(g.lastTimestamp)
		}
	case now > g.lastTimestamp:
		g.sequence = 0
	default:
		// 时钟回拨(NTP 校正 / 手工改时间)。等回拨追平而非直接 panic，
		// 让登录日志这类旁路写入不至于打挂进程；回拨过大时会阻塞，由日志暴露。
		log.Printf("snowflake: 检测到时钟回拨 %d 毫秒，等待追平", g.lastTimestamp-now)
		now = g.waitNextMillis(g.lastTimestamp)
		g.sequence = 0
	}
	g.lastTimestamp = now

	return (now-epoch)<<timestampShift |
		g.datacenterID<<datacenterIDShift |
		g.workerID<<workerIDShift |
		g.sequence
}

// waitNextMillis 自旋等到时间戳超过 last 并返回新时间戳。
func (g *Generator) waitNextMillis(last int64) int64 {
	now := currentMillis()
	for now <= last {
		now = currentMillis()
	}
	return now
}

// currentMillis 返回当前毫秒时间戳。
func currentMillis() int64 {
	return time.Now().UnixMilli()
}

// 包级默认实例。
var (
	mu               sync.RWMutex
	defaultGenerator *Generator
)

// Init 按配置构造生成器并设为包级默认实例。配置非法时 panic。
func Init() {
	cfg := config.Get().Snowflake
	g, err := New(cfg.WorkerID, cfg.DatacenterID)
	if err != nil {
		panic(fmt.Errorf("snowflake: 初始化失败: %w", err))
	}
	mu.Lock()
	defaultGenerator = g
	mu.Unlock()
	log.Printf("snowflake 已就绪: workerId=%d datacenterId=%d", cfg.WorkerID, cfg.DatacenterID)
}

// Next 返回包级默认实例发的下一个 ID，未调用 Init 会 panic。
func Next() int64 {
	mu.RLock()
	g := defaultGenerator
	mu.RUnlock()
	if g == nil {
		panic("snowflake: 尚未初始化，请先调用 snowflake.Init")
	}
	return g.Next()
}
