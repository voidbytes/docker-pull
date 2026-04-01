package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// YAMLConfig 用于解析 YAML 配置文件
type YAMLConfig struct {
	Mirror string `yaml:"mirror"`
	Proxy  string `yaml:"proxy"`
	Arch   string `yaml:"arch"`
	OS     string `yaml:"os"`
}

type Config struct {
	ImageName    string
	OutputDir    string
	MaxRetry     int
	SelectedArch string
	OS           string
	ShowStats    bool
	ProxyURL     string
	Registry     string
}

// NewDefaultConfig 默认配置
func NewDefaultConfig() *Config {
	wd, _ := os.Getwd()
	return &Config{
		OutputDir: wd,
		MaxRetry:  10,
		ShowStats: true,
	}
}

// PrepareOutputDir 初始化输出目录（不存在则创建）
func (c *Config) PrepareOutputDir() error {
	return os.MkdirAll(filepath.Clean(c.OutputDir), 0755)
}

// SetOutputDir 设置自定义输出目录
func (c *Config) SetOutputDir(dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	c.OutputDir = absDir
	return c.PrepareOutputDir()
}

// LoadFromYAML 从 YAML 配置文件加载配置
func (c *Config) LoadFromYAML(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var yamlConfig YAMLConfig
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return err
	}

	if c.Registry == "" {
		c.Registry = yamlConfig.Mirror
	}
	if c.ProxyURL == "" {
		c.ProxyURL = yamlConfig.Proxy
	}
	if c.SelectedArch == "" {
		c.SelectedArch = yamlConfig.Arch
	}
	if c.OS == "" {
		c.OS = strings.ToLower(yamlConfig.OS)
	} else {
		c.OS = strings.ToLower(c.OS)
	}

	return nil
}
