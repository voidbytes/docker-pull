package archive

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"docker-pull/internal/config"
	"docker-pull/internal/download"
	"docker-pull/internal/registry"
)

// PackageToTar 将镜像层打包为tar文件
func PackageToTar(cfg *config.Config, layers []download.ImageLayer, imageName, tag, arch, osName string, configDigest string, regClient *registry.RegistryClient) (string, error) {
	tmpDir := filepath.Join(cfg.OutputDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}
	imgDir := filepath.Join(tmpDir, "tmp_image")
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		return "", err
	}
	defer func() {
		if err := os.RemoveAll(imgDir); err != nil {
			log.Printf("警告: 无法删除临时目录 %s: %v", imgDir, err)
		}
	}()

	if err := buildImageStructure(imgDir, layers, imageName, tag, configDigest, regClient); err != nil {
		return "", err
	}

	safeName := strings.ReplaceAll(imageName, "/", "_")
	safeName = strings.Replace(safeName, ":", "-", 1)
	outputFileName := fmt.Sprintf("%s_%s_%s_%s.tar", safeName, tag, osName, arch)
	outputPath := filepath.Join(cfg.OutputDir, outputFileName)

	if err := createTarFile(imgDir, outputPath); err != nil {
		return "", err
	}

	fmt.Printf("镜像已打包为: %s\n", outputPath)
	return outputPath, nil
}

// buildImageStructure 构建镜像目录结构
func buildImageStructure(imgDir string, layers []download.ImageLayer, imageName, tag string, configDigest string, regClient *registry.RegistryClient) error {
	repoName := imageName
	if strings.Contains(imageName, "/") {
		repoParts := strings.Split(imageName, "/")
		repoName = strings.Join(repoParts[:len(repoParts)-1], "/")
	}

	parentID := ""
	layerPaths := make([]string, len(layers))
	layerJSONMap := make(map[string]map[string]interface{})

	for i, layer := range layers {
		layerID := layer.Digest[7:]
		layerDir := filepath.Join(imgDir, layerID)
		if err := os.MkdirAll(layerDir, 0755); err != nil {
			return err
		}

		layerTarPath := filepath.Join(layerDir, "layer.tar")
		if err := copyFile(layer.Path, layerTarPath); err != nil {
			return err
		}

		layerJSON := map[string]interface{}{
			"id":     layerID,
			"parent": parentID,
		}
		layerJSONPath := filepath.Join(layerDir, "json")
		if err := writeJSON(layerJSONPath, layerJSON); err != nil {
			return err
		}

		layerPaths[i] = fmt.Sprintf("%s/layer.tar", layerID)
		layerJSONMap[layerID] = layerJSON
		parentID = layerID
	}

	manifest := []map[string]interface{}{
		{
			"Config":   "config.json",
			"RepoTags": []string{fmt.Sprintf("%s:%s", imageName, tag)},
			"Layers":   layerPaths,
		},
	}
	manifestPath := filepath.Join(imgDir, "manifest.json")
	if err := writeJSON(manifestPath, manifest); err != nil {
		return err
	}

	repositories := map[string]map[string]string{
		repoName: {tag: parentID},
	}
	repositoriesPath := filepath.Join(imgDir, "repositories")
	if err := writeJSON(repositoriesPath, repositories); err != nil {
		return err
	}

	configPath := filepath.Join(imgDir, "config.json")
	if err := downloadConfig(configPath, configDigest, regClient); err != nil {
		return err
	}

	return nil
}

// createTarFile 创建tar文件
func createTarFile(imgDir, outputPath string) error {
	tarFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := tarFile.Close(); err != nil {
			log.Printf("警告: 无法关闭tar文件: %v", err)
		}
	}()

	tw := tar.NewWriter(tarFile)
	defer func() {
		if err := tw.Close(); err != nil {
			log.Printf("警告: 无法关闭tar写入器: %v", err)
		}
	}()

	return filepath.Walk(imgDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == imgDir {
			return nil
		}

		relPath, err := filepath.Rel(imgDir, path)
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, relPath)
		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer func() {
				if err := file.Close(); err != nil {
					log.Printf("警告: 无法关闭文件 %s: %v", path, err)
				}
			}()

			_, err = io.Copy(tw, file)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := srcFile.Close(); err != nil {
			log.Printf("警告: 无法关闭源文件 %s: %v", src, err)
		}
	}()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if err := dstFile.Close(); err != nil {
			log.Printf("警告: 无法关闭目标文件 %s: %v", dst, err)
		}
	}()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// writeJSON 写入JSON文件
func writeJSON(path string, data interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("警告: 无法关闭文件 %s: %v", path, err)
		}
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// downloadConfig 下载镜像配置文件
func downloadConfig(outputPath, configDigest string, regClient *registry.RegistryClient) error {
	repoPath := regClient.GetRepository()
	url := fmt.Sprintf("%s/v2/%s/blobs/%s", regClient.BaseURL(), repoPath, configDigest)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %v", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", regClient.Token()))

	resp, err := regClient.HTTPClient().Do(req)
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载配置文件失败: HTTP %d", resp.StatusCode)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("警告: 无法关闭文件 %s: %v", outputPath, err)
		}
	}()

	_, err = io.Copy(file, resp.Body)
	return err
}
