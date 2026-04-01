package verify

import (
	"os"
	"testing"
)

func TestSplitDigest(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "正常格式",
			input:    "sha256:abc123",
			expected: []string{"sha256", "abc123"},
		},
		{
			name:     "无冒号",
			input:    "sha256abc123",
			expected: []string{"sha256abc123"},
		},
		{
			name:     "多个冒号",
			input:    "sha256:abc:123",
			expected: []string{"sha256", "abc:123"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := splitDigest(tc.input)
			if len(result) != len(tc.expected) {
				t.Errorf("splitDigest() length = %v, expected %v", len(result), len(tc.expected))
				return
			}
			for i, v := range result {
				if v != tc.expected[i] {
					t.Errorf("splitDigest()[%d] = %v, expected %v", i, v, tc.expected[i])
				}
			}
		})
	}
}

func TestVerifySHA256(t *testing.T) {
	// 创建测试文件
	tmpFile := t.TempDir() + "/test.txt"
	content := "test content"
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// 计算预期的SHA256值
	expectedDigest := "sha256:6ae8a75555209fd6c44157c0aed8016e763ff435a19cf186f76863140143ff72"

	testCases := []struct {
		name     string
		filePath string
		digest   string
		expected bool
		wantErr  bool
	}{
		{
			name:     "正确的SHA256",
			filePath: tmpFile,
			digest:   expectedDigest,
			expected: true,
			wantErr:  false,
		},
		{
			name:     "错误的SHA256",
			filePath: tmpFile,
			digest:   "sha256:wronghash",
			expected: false,
			wantErr:  true,
		},
		{
			name:     "无效的digest格式",
			filePath: tmpFile,
			digest:   "invalidformat",
			expected: false,
			wantErr:  true,
		},
		{
			name:     "文件不存在",
			filePath: "non_existent_file.txt",
			digest:   expectedDigest,
			expected: false,
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := VerifySHA256(tc.filePath, tc.digest)
			if (err != nil) != tc.wantErr {
				t.Errorf("VerifySHA256() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if result != tc.expected {
				t.Errorf("VerifySHA256() = %v, expected %v", result, tc.expected)
			}
		})
	}
}
