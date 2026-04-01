package download

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

// ProgressManager 管理所有下载任务的进度
type ProgressManager struct {
	p     *mpb.Progress
	bars  map[string]*mpb.Bar
	stats *DownloadStats
	mu    sync.Mutex
}

// NewProgressManager 创建一个新的进度管理器
func NewProgressManager() *ProgressManager {
	return &ProgressManager{
		// 增加 mpb.WithOutput(os.Stdout) 强制输出，防止在 IDE 环境下不显示
		p:     mpb.New(mpb.WithWidth(45), mpb.WithRefreshRate(150*time.Millisecond), mpb.WithOutput(os.Stdout)),
		bars:  make(map[string]*mpb.Bar),
		stats: NewDownloadStats(),
	}
}

// AddItem 添加一个下载任务
func (pm *ProgressManager) AddItem(name string, totalSize int64, index, totalLayers int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 避免重复添加
	if _, exists := pm.bars[name]; exists {
		return
	}

	// 定义前缀 (类似 Docker: [1/5] a1b2c3d4e5f6)
	prefix := fmt.Sprintf("[%d/%d] %s", index+1, totalLayers, name)

	bar := pm.p.AddBar(totalSize,
		mpb.PrependDecorators(
			decor.Name(prefix),
			decor.CountersKibiByte("% .2f / % .2f"),
		),
		mpb.AppendDecorators(
			decor.Percentage(),
		),
	)
	pm.bars[name] = bar
}

// UpdateItem 更新下载任务的进度
func (pm *ProgressManager) UpdateItem(name string, downloaded int64) {
	pm.mu.Lock()
	bar, exists := pm.bars[name]
	pm.mu.Unlock()

	if exists {
		bar.SetCurrent(downloaded)
	}
}

// UpdateItemSize 更新下载任务的总大小 (处理接收到 Content-Length 后的情况)
func (pm *ProgressManager) UpdateItemSize(name string, totalSize int64) {
	pm.mu.Lock()
	bar, exists := pm.bars[name]
	pm.mu.Unlock()

	if exists {
		bar.SetTotal(totalSize, false)
	}
}

// CompleteItem 标记下载任务完成
func (pm *ProgressManager) CompleteItem(name string) {
	pm.mu.Lock()
	bar, exists := pm.bars[name]
	pm.mu.Unlock()

	if exists {
		bar.SetTotal(-1, true) // 充满进度条并标记为完成
	}
}

// 下面两个方法保留空实现，以兼容原有的接口调用
func (pm *ProgressManager) SetResume(name string, isResume bool) {}
func (pm *ProgressManager) AddRetry(name string)                 {}

// StartStats 开始统计
func (pm *ProgressManager) StartStats() {
	pm.stats.Start()
}

// AddBytes 累加下载字节数
func (pm *ProgressManager) AddBytes(bytes int64) {
	pm.stats.AddBytes(bytes)
}

// Wait 等待所有进度条渲染完成并清理屏幕（必须在最后调用）
func (pm *ProgressManager) Wait() {
	pm.p.Wait()
}
