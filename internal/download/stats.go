package download

import (
	"fmt"
	"time"
)

type DownloadStats struct {
	startTime  time.Time
	totalBytes int64
}

func NewDownloadStats() *DownloadStats {
	return &DownloadStats{}
}

// Reset 重置统计
func (s *DownloadStats) Reset() {
	*s = DownloadStats{}
}

// Start 开始计时
func (s *DownloadStats) Start() {
	s.startTime = time.Now()
}

// AddBytes 累加下载字节数
func (s *DownloadStats) AddBytes(bytes int64) {
	s.totalBytes += bytes
}

// GetAverageSpeed 获取实时平均速度
func (s *DownloadStats) GetAverageSpeed() float64 {
	totalTime := time.Since(s.startTime).Seconds()
	if totalTime > 0 {
		return float64(s.totalBytes) / totalTime
	}
	return 0
}

// PrintStats 打印下载统计
func (s *DownloadStats) PrintStats() {
	totalTime := time.Since(s.startTime)
	averageSpeed := s.GetAverageSpeed()
	fmt.Printf("\n=== 下载统计 ===\n")
	fmt.Printf("总耗时: %s\n", totalTime.Round(time.Millisecond))
	fmt.Printf("总大小: %.2f MB\n", float64(s.totalBytes)/1024/1024)
	fmt.Printf("平均速度: %.2f MB/s\n", averageSpeed/1024/1024)
}
