package snowflake

import (
	"fmt"
	"log"
	"sync"
	"time"
)

const (
	workerIDBits     = 5
	datacenterIDBits = 5
	sequenceBits     = 12
)

const (
	maxWorkerID     int64 = -1 ^ (-1 << workerIDBits)     // 31
	maxDatacenterID int64 = -1 ^ (-1 << datacenterIDBits) // 31
	sequenceMask    int64 = -1 ^ (-1 << sequenceBits)     // 4095

	workerIDShift     = sequenceBits                                   // 12
	datacenterIDShift = sequenceBits + workerIDBits                    // 17
	timestampShift    = sequenceBits + workerIDBits + datacenterIDBits // 22
)

// epoch 起始毫秒时间戳（2020-01-01 UTC）。改它会让新旧 ID 大小关系错乱，上线后不可再动。
const epoch int64 = 1577836800000

// Generator 雪花 ID 生成器，并发安全。
type Generator struct {
	mu            sync.Mutex
	workerID      int64
	datacenterID  int64
	sequence      int64
	lastTimestamp int64
}

// NewGenerator 构造生成器。cfg 零值即默认配置（workerId=0, datacenterId=0），
// 未配置时传 SnowflakeConfig{} 即可。机器号越界返回错误。
func NewGenerator(cfg SnowflakeConfig) (*Generator, error) {
	if cfg.WorkerID < 0 || cfg.WorkerID > maxWorkerID {
		return nil, fmt.Errorf("snowflake: workerId %d 越界，须在 0-%d 之间", cfg.WorkerID, maxWorkerID)
	}
	if cfg.DatacenterID < 0 || cfg.DatacenterID > maxDatacenterID {
		return nil, fmt.Errorf("snowflake: datacenterId %d 越界，须在 0-%d 之间", cfg.DatacenterID, maxDatacenterID)
	}
	return &Generator{
		workerID:      cfg.WorkerID,
		datacenterID:  cfg.DatacenterID,
		lastTimestamp: -1,
	}, nil
}

// Next 返回下一个 ID。同毫秒序列号用尽会自旋到下一毫秒。
func (g *Generator) Next() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := currentMillis()
	switch {
	case now == g.lastTimestamp:
		// 同毫秒内递增序列号，用尽(4096 个)则等下一毫秒。
		g.sequence = (g.sequence + 1) & sequenceMask
		if g.sequence == 0 {
			now = g.waitNextMillis(g.lastTimestamp)
		}
	case now > g.lastTimestamp:
		g.sequence = 0
	default:
		// 时钟回拨：等追平而非 panic，避免登录日志这类旁路写入打挂进程；
		// 回拨过大会阻塞，由日志暴露。
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

func (g *Generator) waitNextMillis(last int64) int64 {
	now := currentMillis()
	for now <= last {
		now = currentMillis()
	}
	return now
}

func currentMillis() int64 {
	return time.Now().UnixMilli()
}
