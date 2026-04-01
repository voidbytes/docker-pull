package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// VerifySHA256 校验文件的SHA256值（digest格式：sha256:xxxx）
func VerifySHA256(filePath, expectedDigest string) (bool, error) {
	// 解析digest（如sha256:abc123 -> abc123）
	parts := splitDigest(expectedDigest)
	if len(parts) != 2 || parts[0] != "sha256" {
		return false, fmt.Errorf("无效的SHA256 digest格式: %s", expectedDigest)
	}
	expectedHash := parts[1]

	// 读取文件并计算SHA256
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("警告: 无法关闭文件 %s: %v", filePath, err)
		}
	}()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false, err
	}
	actualHash := hex.EncodeToString(hash.Sum(nil))

	// 对比
	if actualHash != expectedHash {
		return false, fmt.Errorf("SHA256校验失败: 预期=%s, 实际=%s", expectedHash, actualHash)
	}
	return true, nil
}

// 辅助函数：拆分digest（sha256:xxxx -> [sha256, xxxx]）
func splitDigest(digest string) []string {
	return strings.SplitN(digest, ":", 2)
}
