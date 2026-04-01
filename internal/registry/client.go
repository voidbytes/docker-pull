package registry

import (
	"docker-pull/internal/config"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// RegistryClient Registry API客户端
type RegistryClient struct {
	client  *http.Client
	config  *config.Config
	baseURL string     // 镜像仓库基础URL（如https://registry-1.docker.io）
	token   string     // 认证token
	imgInfo *ImageName // 存储解析后的镜像信息
}

// NewRegistryClient 初始化客户端
func NewRegistryClient(cfg *config.Config) (*RegistryClient, error) {
	imgInfo, err := ParseImageName(cfg.ImageName)
	if err != nil {
		return nil, fmt.Errorf("解析镜像名称失败: %v", err)
	}

	registryHost := imgInfo.Registry
	if cfg.Registry != "" {
		registryHost = cfg.Registry
	}
	if registryHost == "docker.io" {
		registryHost = "registry-1.docker.io"
	}
	baseURL := fmt.Sprintf("https://%s", registryHost)

	client := &http.Client{
		Timeout: 0,
	}

	if cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.ProxyURL)
		if err == nil {
			client.Transport = &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
			}
		}
	}

	return &RegistryClient{
		client:  client,
		config:  cfg,
		baseURL: baseURL,
		imgInfo: imgInfo,
	}, nil
}

// GetManifestList 获取镜像的多架构Manifest List
func (c *RegistryClient) GetManifestList() (*ManifestList, error) {
	token, err := c.getAuthToken()
	if err != nil {
		fmt.Printf("⚠️ 获取Token提示: %v\n", err)
	}
	c.token = token

	ref := c.imgInfo.Tag
	if c.imgInfo.Digest != "" {
		ref = c.imgInfo.Digest
	}

	urlStr := fmt.Sprintf("%s/v2/%s/manifests/%s", c.baseURL, c.imgInfo.Repository, ref)
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.docker.distribution.manifest.v2+json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("警告: 无法关闭HTTP响应体: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求 manifest 失败，状态码: %d", resp.StatusCode)
	}

	var manifestList ManifestList
	if err := json.NewDecoder(resp.Body).Decode(&manifestList); err != nil {
		return nil, err
	}
	return &manifestList, nil
}

// getAuthToken 获取认证Token
func (c *RegistryClient) getAuthToken() (string, error) {
	probeURL := fmt.Sprintf("%s/v2/", c.baseURL)
	probeReq, err := http.NewRequest("GET", probeURL, nil)
	if err != nil {
		return "", err
	}

	probeResp, err := c.client.Do(probeReq)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := probeResp.Body.Close(); err != nil {
			log.Printf("警告: 无法关闭HTTP响应体: %v", err)
		}
	}()

	if probeResp.StatusCode == http.StatusOK {
		return "", nil
	}
	if probeResp.StatusCode != http.StatusUnauthorized {
		return "", fmt.Errorf("探测状态异常: %d", probeResp.StatusCode)
	}

	authHeader := probeResp.Header.Get("WWW-Authenticate")
	if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return "", fmt.Errorf("不支持的认证类型: %s", authHeader)
	}

	parts := strings.Split(authHeader[7:], ",")
	var realm, service string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "realm=") {
			realm = strings.Trim(p[6:], `"`)
		} else if strings.HasPrefix(p, "service=") {
			service = strings.Trim(p[8:], `"`)
		}
	}

	if realm == "" {
		return "", fmt.Errorf("解析 realm 失败")
	}

	authURL := fmt.Sprintf("%s?service=%s&scope=repository:%s:pull", realm, service, c.imgInfo.Repository)
	authReq, err := http.NewRequest("GET", authURL, nil)
	if err != nil {
		return "", err
	}

	authResp, err := c.client.Do(authReq)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := authResp.Body.Close(); err != nil {
			log.Printf("警告: 无法关闭HTTP响应体: %v", err)
		}
	}()

	if authResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("拉取Token失败: %d", authResp.StatusCode)
	}

	var authResult struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(authResp.Body).Decode(&authResult); err != nil {
		return "", err
	}

	if authResult.Token != "" {
		return authResult.Token, nil
	}
	return authResult.AccessToken, nil
}

// BaseURL 返回镜像仓库基础URL
func (c *RegistryClient) BaseURL() string {
	return c.baseURL
}

// Token 返回认证token
func (c *RegistryClient) Token() string {
	return c.token
}

// HTTPClient 返回HTTP客户端
func (c *RegistryClient) HTTPClient() *http.Client {
	return c.client
}

// GetManifest 根据特定的 digest 获取该架构的详细 Manifest
func (c *RegistryClient) GetManifest(digest string) (*Manifest, error) {
	urlStr := fmt.Sprintf("%s/v2/%s/manifests/%s", c.baseURL, c.imgInfo.Repository, digest)
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	if c.token != "" && c.token != "dummy-token" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("警告: 无法关闭HTTP响应体: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求 specific manifest 失败，状态码: %d", resp.StatusCode)
	}

	var manifest Manifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}
func (c *RegistryClient) GetRepository() string {
	return c.imgInfo.Repository
}

// ValidateAndNormalizeRegistry 校验并标准化镜像仓库地址
func ValidateAndNormalizeRegistry(registry string) (string, error) {
	if registry == "" {
		return "", nil
	}

	registry = strings.TrimSuffix(registry, "/")

	if !strings.HasPrefix(registry, "http://") && !strings.HasPrefix(registry, "https://") {
		return registry, nil
	}

	parsed, err := url.Parse(registry)
	if err != nil {
		return registry, nil
	}

	host := parsed.Host
	if host == "" {
		host = parsed.Path
	}

	return host, nil
}
