package registry

import (
	"testing"
)

func TestValidateAndNormalizeRegistry(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "空字符串",
			input:    "",
			expected: "",
			wantErr:  false,
		},
		{
			name:     "普通域名",
			input:    "registry.example.com",
			expected: "registry.example.com",
			wantErr:  false,
		},
		{
			name:     "带端口的域名",
			input:    "registry.example.com:5000",
			expected: "registry.example.com:5000",
			wantErr:  false,
		},
		{
			name:     "带http前缀",
			input:    "http://registry.example.com",
			expected: "registry.example.com",
			wantErr:  false,
		},
		{
			name:     "带https前缀",
			input:    "https://registry.example.com",
			expected: "registry.example.com",
			wantErr:  false,
		},
		{
			name:     "带https前缀和端口",
			input:    "https://registry.example.com:5000",
			expected: "registry.example.com:5000",
			wantErr:  false,
		},
		{
			name:     "带末尾斜杠",
			input:    "registry.example.com/",
			expected: "registry.example.com",
			wantErr:  false,
		},
		{
			name:     "带https前缀和末尾斜杠",
			input:    "https://registry.example.com:5000/",
			expected: "registry.example.com:5000",
			wantErr:  false,
		},
		{
			name:     "localhost",
			input:    "localhost",
			expected: "localhost",
			wantErr:  false,
		},
		{
			name:     "localhost带端口",
			input:    "localhost:5000",
			expected: "localhost:5000",
			wantErr:  false,
		},
		{
			name:     "带路径的URL",
			input:    "https://registry.example.com/path",
			expected: "registry.example.com",
			wantErr:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ValidateAndNormalizeRegistry(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateAndNormalizeRegistry() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if result != tc.expected {
				t.Errorf("ValidateAndNormalizeRegistry() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestParseImageName(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected *ImageName
		wantErr  bool
	}{
		{
			name:    "空字符串",
			input:   "",
			wantErr: true,
		},
		{
			name:  "简单镜像名",
			input: "nginx",
			expected: &ImageName{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "latest",
				Digest:     "",
			},
			wantErr: false,
		},
		{
			name:  "带标签的镜像名",
			input: "nginx:1.24",
			expected: &ImageName{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "1.24",
				Digest:     "",
			},
			wantErr: false,
		},
		{
			name:  "带摘要的镜像名",
			input: "nginx@sha256:123456",
			expected: &ImageName{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "",
				Digest:     "sha256:123456",
			},
			wantErr: false,
		},
		{
			name:  "带自定义仓库的镜像名",
			input: "my-registry:5000/app:v1",
			expected: &ImageName{
				Registry:   "my-registry:5000",
				Repository: "app",
				Tag:        "v1",
				Digest:     "",
			},
			wantErr: false,
		},
		{
			name:  "带命名空间的镜像名",
			input: "my-project/app:v1",
			expected: &ImageName{
				Registry:   "docker.io",
				Repository: "my-project/app",
				Tag:        "v1",
				Digest:     "",
			},
			wantErr: false,
		},
		{
			name:  "官方仓库完整路径",
			input: "docker.io/library/nginx",
			expected: &ImageName{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "latest",
				Digest:     "",
			},
			wantErr: false,
		},
		{
			name:  "localhost仓库",
			input: "localhost:5000/app",
			expected: &ImageName{
				Registry:   "localhost:5000",
				Repository: "app",
				Tag:        "latest",
				Digest:     "",
			},
			wantErr: false,
		},
		{
			name:    "多个@分隔符",
			input:   "nginx@sha256:123@456",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseImageName(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("ParseImageName() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			if !tc.wantErr {
				if result.Registry != tc.expected.Registry {
					t.Errorf("ParseImageName() Registry = %v, expected %v", result.Registry, tc.expected.Registry)
				}
				if result.Repository != tc.expected.Repository {
					t.Errorf("ParseImageName() Repository = %v, expected %v", result.Repository, tc.expected.Repository)
				}
				if result.Tag != tc.expected.Tag {
					t.Errorf("ParseImageName() Tag = %v, expected %v", result.Tag, tc.expected.Tag)
				}
				if result.Digest != tc.expected.Digest {
					t.Errorf("ParseImageName() Digest = %v, expected %v", result.Digest, tc.expected.Digest)
				}
			}
		})
	}
}
