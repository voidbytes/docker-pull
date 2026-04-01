package download

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"docker-pull/internal/config"
	"docker-pull/internal/registry"
	"docker-pull/internal/verify"
)

type Downloader struct {
	config     *config.Config
	regClient  *registry.RegistryClient
	progress   *ProgressManager
	maxRetry   int
	retryDelay time.Duration // 重试退避延迟（指数退避）
}

// ImageLayer 表示镜像层信息
type ImageLayer struct {
	Digest string
	Path   string
}

// progressWriter 实现io.Writer接口，用于跟踪下载进度
type progressWriter struct {
	file       *os.File
	downloaded *int64
	progress   *ProgressManager
	digest     string
}

// Write 实现io.Writer接口
func (pw *progressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.file.Write(p)
	if err != nil {
		return
	}
	*pw.downloaded += int64(n)
	if pw.progress != nil {
		pw.progress.UpdateItem(pw.digest, *pw.downloaded)
		pw.progress.AddBytes(int64(n))
	}
	return
}

func NewDownloader(cfg *config.Config, regClient *registry.RegistryClient) *Downloader {
	return &Downloader{
		config:     cfg,
		regClient:  regClient,
		maxRetry:   cfg.MaxRetry,
		retryDelay: 1 * time.Second, // 初始重试延迟
	}
}

// SetProgressManager 设置进度管理器
func (d *Downloader) SetProgressManager(pm *ProgressManager) {
	d.progress = pm
}

// DownloadLayer 下载镜像层（无限重试）
func (d *Downloader) DownloadLayer(digest, layerName string) (string, error) {
	tmpDir := filepath.Join(d.config.OutputDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}
	outputPath := filepath.Join(tmpDir, layerName)

	var err error
	for retry := 0; ; retry++ {
		err = d.downloadLayerOnce(digest, outputPath)
		if err == nil {
			break
		} else {
			log.Printf("Failed to download layer %s: %v, retrying...", layerName, err)
		}
		delay := d.retryDelay * time.Duration(retry+1)
		if delay > 10*time.Second {
			delay = 10 * time.Second
		}
		if d.progress != nil {
			d.progress.AddRetry(digest[:12])
		}
		time.Sleep(delay)
	}

	if d.progress != nil {
		d.progress.CompleteItem(digest[:12])
	}

	return outputPath, nil
}

// DownloadLayers 并发下载多个镜像层
func (d *Downloader) DownloadLayers(digests []string) ([]ImageLayer, error) {
	fmt.Printf("识别到 %d 个镜像层，开始并发下载 (最大并发: 5)...\n", len(digests))

	var wg sync.WaitGroup
	var mu sync.Mutex
	layers := make([]ImageLayer, len(digests))
	errChan := make(chan error, len(digests))

	sem := make(chan struct{}, 5)

	if d.progress != nil {
		d.progress.StartStats()
	}

	for i, digest := range digests {
		wg.Add(1)
		go func(index int, dgst string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			layerName := fmt.Sprintf("layer-%d-%s", index, dgst[7:19])
			subDownloader := NewDownloader(d.config, d.regClient)
			subDownloader.SetProgressManager(d.progress)

			if d.progress != nil {
				d.progress.AddItem(dgst[:12], 0, index, len(digests))
			}

			path, err := subDownloader.DownloadLayer(dgst, layerName)
			if err != nil {
				if d.progress != nil {
					d.progress.CompleteItem(dgst[:12])
				}
				errChan <- fmt.Errorf("层 %s 下载失败: %v", dgst[:12], err)
				return
			}

			ok, err := verify.VerifySHA256(path, dgst)
			if err != nil || !ok {
				errChan <- fmt.Errorf("层 %s 校验未通过", dgst[:12])
				return
			}

			mu.Lock()
			layers[index] = ImageLayer{
				Digest: dgst,
				Path:   path,
			}
			mu.Unlock()
		}(i, digest)
	}

	wg.Wait()
	close(errChan)

	if d.progress != nil {
		d.progress.Wait()
	}

	if len(errChan) > 0 {
		errors := make([]string, 0)
		for e := range errChan {
			errors = append(errors, e.Error())
		}
		return nil, fmt.Errorf("部分镜像层下载失败: %v", errors)
	}

	finalLayers := make([]ImageLayer, 0)
	for _, layer := range layers {
		if layer.Path != "" {
			finalLayers = append(finalLayers, layer)
		}
	}

	return finalLayers, nil
}

// 单次下载逻辑
func (d *Downloader) downloadLayerOnce(digest, outputPath string) error {
	var resumePos int64 = 0
	if fileInfo, err := os.Stat(outputPath); err == nil {
		resumePos = fileInfo.Size()
	}

	repoPath := d.regClient.GetRepository()
	url := fmt.Sprintf("%s/v2/%s/blobs/%s", d.regClient.BaseURL(), repoPath, digest)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.regClient.Token()))

	if resumePos > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumePos))
		if d.progress != nil {
			d.progress.SetResume(digest[:12], true)
		}
	}

	resp, err := d.regClient.HTTPClient().Do(req)
	if err != nil {
		return err
	}
	if resp == nil {
		return fmt.Errorf("HTTP响应为nil")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("警告: 无法关闭HTTP响应体: %v", err)
		}
	}()

	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		// 处理416错误：范围不可满足，重置为从头下载
		resumePos = 0
		if err := os.Remove(outputPath); err != nil {
			log.Printf("警告: 无法删除文件 %s: %v", outputPath, err)
		}
		// 重新发起请求
		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("创建HTTP请求失败: %v", err)
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.regClient.Token()))
		resp, err = d.regClient.HTTPClient().Do(req)
		if err != nil {
			return err
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.Printf("警告: 无法关闭HTTP响应体: %v", err)
			}
		}()
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusOK && resumePos > 0 {
		resumePos = 0
		if err := os.Remove(outputPath); err != nil {
			log.Printf("警告: 无法删除文件 %s: %v", outputPath, err)
		}
	}

	openFlags := os.O_CREATE | os.O_WRONLY
	if resumePos > 0 {
		openFlags |= os.O_APPEND
	} else {
		openFlags |= os.O_TRUNC
	}

	file, err := os.OpenFile(outputPath, openFlags, 0644)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("警告: 无法关闭文件 %s: %v", outputPath, err)
		}
	}()

	totalSize := resp.ContentLength + resumePos
	if d.progress != nil {
		d.progress.UpdateItemSize(digest[:12], totalSize)
	}

	downloaded := resumePos
	writer := &progressWriter{
		file:       file,
		downloaded: &downloaded,
		progress:   d.progress,
		digest:     digest[:12],
	}

	_, err = io.Copy(writer, resp.Body)
	return err
}
