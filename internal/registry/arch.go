package registry

import (
	"fmt"
	"strings"

	"github.com/manifoldco/promptui"
)

// ListArchitectures 提取镜像可用的架构列表
func (c *RegistryClient) ListArchitectures(ml *ManifestList) []string {
	var archs []string
	for _, m := range ml.Manifests {
		arch := m.Platform.Architecture
		if !contains(archs, arch) {
			archs = append(archs, arch)
		}
	}
	return archs
}

// PromptArchSelection 交互式选择架构
func (c *RegistryClient) PromptArchSelection(archs []string) (string, error) {
	if len(archs) == 1 {
		return archs[0], nil // 只有一种架构，直接返回
	}

	// 交互式选择
	prompt := promptui.Select{
		Label: "请选择镜像架构",
		Items: archs,
	}
	_, selected, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("架构选择失败: %v", err)
	}
	return selected, nil
}

// 辅助函数：判断切片是否包含元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}
