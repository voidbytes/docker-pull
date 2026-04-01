package registry

import (
	"errors"
	"strings"
)

// ImageName 解析后的镜像名称结构
type ImageName struct {
	Registry   string // 镜像仓库域名（含端口），如 docker.io、192.168.1.100:5000
	Repository string // 镜像仓库名（含命名空间），如 library/nginx、my-project/app
	Tag        string // 镜像标签（默认 latest）
	Digest     string // 镜像摘要（如 sha256:xxxx，优先级高于 Tag）
}

// 私有常量：Docker 官方仓库默认值
const (
	defaultRegistry = "docker.io"
	defaultTag      = "latest"
	officialNS      = "library/" // Docker 官方库默认命名空间
)

// ParseImageName 解析 Docker 镜像名称，返回结构化结果
// 支持的镜像名格式示例：
// - nginx → docker.io/library/nginx:latest
// - nginx:1.24 → docker.io/library/nginx:1.24
// - my-registry:5000/app:v1 → my-registry:5000/app:v1
// - docker.io/library/nginx@sha256:xxx → docker.io/library/nginx@sha256:xxx
// - gcr.io/google-containers/pause → gcr.io/google-containers/pause:latest
func ParseImageName(image string) (*ImageName, error) {
	if strings.TrimSpace(image) == "" {
		return nil, errors.New("镜像名称不能为空")
	}

	img := &ImageName{
		Tag: defaultTag, // 默认标签 latest
	}

	// 1. 拆分摘要（@ 分隔，优先级高于 Tag）
	parts := strings.Split(image, "@")
	if len(parts) == 2 {
		img.Digest = parts[1]
		image = parts[0] // 剩余部分处理 Tag 和仓库
		img.Tag = ""     // 有摘要时标签无效
	} else if len(parts) > 2 {
		return nil, errors.New("镜像名称包含多个 @ 分隔符，格式非法")
	}

	// 2. 拆分标签（: 分隔）
	// 注意：域名可能包含端口（如 192.168.1.100:5000），需区分端口和标签的 :
	tagParts := strings.Split(image, ":")
	var repoPart string
	if len(tagParts) > 1 {
		// 判断最后一个 : 是不是端口（域名部分）：含数字/IP 且无 / 则为端口，否则为标签
		lastColonIdx := strings.LastIndex(image, ":")
		domainPart := image[:lastColonIdx]
		tagCandidate := image[lastColonIdx+1:]

		// 规则：如果 domainPart 包含 / → 最后一个 : 是标签；否则检查是否为合法端口（数字）
		if strings.Contains(domainPart, "/") {
			// 示例：library/nginx:1.24 → 标签 1.24
			repoPart = domainPart
			img.Tag = tagCandidate
		} else {
			// 检查 tagCandidate 是否包含 /，如果包含则说明是路径，不是端口
			if strings.Contains(tagCandidate, "/") {
				// 包含 /，说明是路径，如 localhost:5000/app → 完整作为仓库部分
				repoPart = image
			} else {
				// 无 /，判断是否为端口（纯数字）
				isPort := true
				for _, c := range tagCandidate {
					if c < '0' || c > '9' {
						isPort = false
						break
					}
				}
				if isPort {
					// 是端口，如 192.168.1.100:5000 → 完整作为仓库域名
					repoPart = image
				} else {
					// 是标签，如 nginx:1.24 → 域名+仓库为 nginx，标签 1.24
					repoPart = domainPart
					img.Tag = tagCandidate
				}
			}
		}
	} else {
		// 无标签，如 nginx → 完整作为仓库部分
		repoPart = image
	}

	// 3. 拆分仓库域名和仓库名（/ 分隔）
	repoParts := strings.Split(repoPart, "/")
	if len(repoParts) == 1 {
		// 无命名空间，如 nginx → 补充官方命名空间 library/
		img.Registry = defaultRegistry
		img.Repository = officialNS + repoParts[0]
	} else {
		// 判断第一部分是否为仓库域名（含 . 或 :，或为 localhost）
		firstPart := repoParts[0]
		isRegistry := strings.Contains(firstPart, ".") || strings.Contains(firstPart, ":") || firstPart == "localhost"

		if isRegistry {
			// 第一部分是域名，如 docker.io/library/nginx → 域名 docker.io，仓库 library/nginx
			img.Registry = firstPart
			img.Repository = strings.Join(repoParts[1:], "/")
		} else {
			// 无自定义域名，如 my-project/app → 域名 docker.io，仓库 my-project/app
			img.Registry = defaultRegistry
			img.Repository = strings.Join(repoParts, "/")
		}

		// 特殊处理：docker.io 下的无命名空间仓库（如 docker.io/nginx → 转为 docker.io/library/nginx）
		if img.Registry == defaultRegistry && !strings.Contains(img.Repository, "/") {
			img.Repository = officialNS + img.Repository
		}
	}

	// 校验仓库名合法性（简化版，可根据需求扩展）
	if strings.ContainsAny(img.Repository, `\<>:"|?*`) {
		return nil, errors.New("镜像仓库名包含非法字符")
	}

	return img, nil
}
