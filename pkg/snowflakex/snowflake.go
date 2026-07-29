package snowflakex

import (
	"fmt"
	"sync"
	"time"
)

// SnowflakeIDGenerator 雪花算法 ID 生成器
type SnowflakeIDGenerator struct {
	mu           sync.Mutex
	epoch        int64 // 起始时间戳（毫秒）
	workerID     int64 // 工作节点 ID（0-31）
	datacenterID int64 // 数据中心 ID（0-31）
	sequence     int64 // 序列号（0-4095）
	lastTime     int64 // 上次生成 ID 的时间戳
}

// NewSnowflakeIDGenerator 创建雪花算法生成器
// workerID: 工作节点 ID (0-31)
// datacenterID: 数据中心 ID (0-31)
func NewSnowflakeIDGenerator(workerID, datacenterID int64) *SnowflakeIDGenerator {
	// 自定义 epoch（2024-01-01 00:00:00）
	epoch := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()

	return &SnowflakeIDGenerator{
		epoch:        epoch,
		workerID:     workerID,
		datacenterID: datacenterID,
		sequence:     0,
		lastTime:     0,
	}
}

// Generate 生成 10 位字符串 ID
func (s *SnowflakeIDGenerator) Generate() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()

	if now < s.lastTime {
		// 时钟回拨，等待
		time.Sleep(time.Duration(s.lastTime-now) * time.Millisecond)
		now = time.Now().UnixMilli()
	}

	if now == s.lastTime {
		// 同一毫秒内，序列号自增
		s.sequence = (s.sequence + 1) & 4095
		if s.sequence == 0 {
			// 序列号溢出，等待下一毫秒
			for now <= s.lastTime {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		// 不同毫秒，序列号重置
		s.sequence = 0
	}

	s.lastTime = now

	// 生成 64 位 ID
	id := ((now - s.epoch) << 22) | (s.datacenterID << 17) | (s.workerID << 12) | s.sequence

	// 转换为 10 位字符串（Base36 编码）
	return fmt.Sprintf("%010s", toBase36(id))
}

// toBase36 将 int64 转换为 Base36 字符串
func toBase36(num int64) string {
	const base36 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if num == 0 {
		return "0"
	}

	var result []byte
	for num > 0 {
		result = append([]byte{base36[num%36]}, result...)
		num /= 36
	}

	return string(result)
}

// 全局 TaskID 生成器（单例）
var taskIDGenerator *SnowflakeIDGenerator
var once sync.Once

// InitTaskIDGenerator 初始化全局 TaskID 生成器
func InitTaskIDGenerator(workerID, datacenterID int64) {
	once.Do(func() {
		taskIDGenerator = NewSnowflakeIDGenerator(workerID, datacenterID)
	})
}

// GenerateTaskID 生成任务 ID
func GenerateTaskID() string {
	if taskIDGenerator == nil {
		// 默认配置（workerID=1, datacenterID=1）
		InitTaskIDGenerator(1, 1)
	}
	return taskIDGenerator.Generate()
}
