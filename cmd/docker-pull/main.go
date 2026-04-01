package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"docker-pull/internal/archive"
	"docker-pull/internal/config"
	"docker-pull/internal/download"
	"docker-pull/internal/registry"
)

func main() {
	var outputDir string
	var arch string
	var osName string
	var proxyURL string
	var mirror string
	flag.StringVar(&outputDir, "o", "./output", "输出目录")
	flag.StringVar(&outputDir, "output", "./output", "输出目录")
	flag.StringVar(&arch, "a", "", "架构")
	flag.StringVar(&arch, "arch", "", "架构")
	flag.StringVar(&osName, "os", "", "操作系统")
	flag.StringVar(&proxyURL, "p", "", "代理URL")
	flag.StringVar(&proxyURL, "proxy", "", "代理URL")
	flag.StringVar(&mirror, "mirror", "", "自定义镜像仓库地址")
	flag.StringVar(&mirror, "m", "", "自定义镜像仓库地址")
	flag.Parse()

	if proxyURL == "" {
		proxyURL = os.Getenv("HTTP_PROXY")
		if proxyURL == "" {
			proxyURL = os.Getenv("HTTPS_PROXY")
			if proxyURL == "" {
				proxyURL = os.Getenv("http_proxy")
				if proxyURL == "" {
					proxyURL = os.Getenv("https_proxy")
				}
			}
		}
	}
	if proxyURL != "" {
		if _, err := url.Parse(proxyURL); err != nil {
			fmt.Printf("代理URL格式无效: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("使用代理URL: %s\n", proxyURL)
	}

	if mirror != "" {
		normalizedMirror, err := registry.ValidateAndNormalizeRegistry(mirror)
		if err != nil {
			fmt.Printf("镜像仓库地址格式无效: %v\n", err)
			os.Exit(1)
		}
		mirror = normalizedMirror
		fmt.Printf("使用自定义镜像仓库: %s\n", mirror)
	}

	if len(flag.Args()) == 0 {
		fmt.Println("用法: docker-pull [选项] <镜像名称>")
		fmt.Println("选项:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	cfg := config.NewDefaultConfig()
	cfg.ImageName = flag.Args()[0]
	cfg.OutputDir = outputDir
	cfg.ProxyURL = proxyURL
	cfg.OS = osName
	cfg.Registry = mirror

	configFile := ""
	if _, err := os.Stat("./config.yml"); err == nil {
		configFile = "./config.yml"
	} else if _, err := os.Stat("./config.yaml"); err == nil {
		configFile = "./config.yaml"
	}
	if configFile != "" {
		if err := cfg.LoadFromYAML(configFile); err != nil {
			fmt.Printf("加载配置文件失败: %v\n", err)
		}
	}

	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		fmt.Printf("创建输出目录失败: %v\n", err)
		os.Exit(1)
	}

	regClient, err := registry.NewRegistryClient(cfg)
	if err != nil {
		fmt.Printf("初始化Registry客户端失败: %v\n", err)
		os.Exit(1)
	}

	manifestList, err := regClient.GetManifestList()
	if err != nil {
		fmt.Printf("获取Manifest List失败: %v\n", err)
		os.Exit(1)
	}
	archs := regClient.ListArchitectures(manifestList)
	if len(archs) == 0 {
		fmt.Println("未识别到可用架构")
		os.Exit(1)
	}

	selectedArch := arch
	if selectedArch == "" && cfg.SelectedArch == "" {
		selectedArch = archs[0]
		fmt.Printf("未指定架构，自动选择: %s\n", selectedArch)
	} else {
		if selectedArch == "" {
			selectedArch = cfg.SelectedArch
		}
		found := false
		for _, a := range archs {
			if a == selectedArch {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("指定的架构 %s 不可用，可用架构: %v\n", selectedArch, archs)
			os.Exit(1)
		}
	}
	cfg.SelectedArch = selectedArch
	fmt.Printf("使用架构: %s\n", selectedArch)

	if osName == "" {
		if cfg.OS == "" {
			osName = "linux"
			fmt.Printf("未指定操作系统，自动选择: %s\n", osName)
		} else {
			osName = cfg.OS
		}

	}
	cfg.OS = strings.ToLower(osName)
	fmt.Printf("使用操作系统: %s\n", cfg.OS)

	var targetDigest string
	for _, m := range manifestList.Manifests {
		if m.Platform.Architecture == cfg.SelectedArch && m.Platform.OS == cfg.OS {
			targetDigest = m.Digest
			break
		}
	}

	if targetDigest == "" {
		fmt.Printf("未找到架构 %s 和操作系统 %s 对应的 Manifest\n", cfg.SelectedArch, cfg.OS)
		os.Exit(1)
	}

	manifest, err := regClient.GetManifest(targetDigest)
	if err != nil {
		fmt.Printf("获取架构 Manifest 失败: %v\n", err)
		os.Exit(1)
	}

	configDigest := manifest.Config.Digest

	layerDigests := make([]string, 0)
	for _, layer := range manifest.Layers {
		layerDigests = append(layerDigests, layer.Digest)
	}

	downloader := download.NewDownloader(cfg, regClient)
	progressManager := download.NewProgressManager()
	downloader.SetProgressManager(progressManager)
	imageLayers, err := downloader.DownloadLayers(layerDigests)
	if err != nil {
		fmt.Printf("下载失败: %v\n", err)
		os.Exit(1)
	}

	imgName := cfg.ImageName
	tag := "latest"
	if strings.Contains(cfg.ImageName, ":") {
		parts := strings.Split(cfg.ImageName, ":")
		imgName = parts[0]
		tag = parts[1]
	}

	fmt.Printf("\n正在打包镜像...\n")

	_, err = archive.PackageToTar(cfg, imageLayers, imgName, tag, selectedArch, osName, configDigest, regClient)
	if err != nil {
		fmt.Printf("打包失败: %v\n", err)
		os.Exit(1)
	}

	for _, layer := range imageLayers {
		if layer.Path != "" {
			if err := os.Remove(layer.Path); err != nil {
				log.Printf("警告: 无法删除临时文件 %s: %v", layer.Path, err)
			}
		}
	}
	fmt.Println("✅ 镜像拉取并打包完成！")
}
