# docker-pull

一个轻量级的 Docker 镜像拉取工具，支持多架构选择、并发下载和断点续传，适用于在没有 Docker 环境的情况下拉取和打包 Docker 镜像。

## 功能特性

- ✅ 支持标准 Docker 镜像名称格式（如 `nginx:latest`、`ubuntu@sha256:xxx`）
- ✅ 自动检测并支持多架构镜像（如 amd64、arm64 等）
- ✅ 并发下载镜像层，提高下载速度
- ✅ 支持断点续传，网络中断后可继续下载
- ✅ 支持代理设置（命令行参数、环境变量或配置文件）
- ✅ 支持指定自定义镜像仓库地址（命令行参数或配置文件）
- ✅ 支持 YAML 配置文件（config.yml）
- ✅ 生成标准的 Docker 镜像 tar 包，可直接使用 `docker load` 加载
- ✅ 详细的下载进度显示
- ✅ 支持指定操作系统（默认为 linux）
- ✅ OS 名称大小写不敏感（自动标准化为小写）

## 安装方法

### 从源码构建

1. 确保已安装 Go 1.26 或更高版本
2. 克隆仓库：
   ```bash
   git clone https://github.com/yourusername/docker-pull.git
   cd docker-pull
   ```
3. 构建可执行文件：
   ```bash
   go build -o docker-pull ./cmd/docker-pull
   ```
4. 将可执行文件添加到系统路径中

## 使用方法

### 基本用法

```bash
docker-pull [选项] <镜像名称>
```

### 选项说明

| 选项 | 别名 | 描述 | 默认值 |
|------|------|------|--------|
| `--output` | `-o` | 输出目录 | `./output` |
| `--arch` | `-a` | 架构（如 amd64、arm64） | 自动选择 |
| `--os` | | 操作系统 | `linux` |
| `--proxy` | `-p` | 代理 URL | 从环境变量获取 |
| `--mirror` | `-m` | 自定义镜像仓库地址 | 从镜像名称解析 |

### 示例

1. 拉取最新版本的 nginx 镜像：
   ```bash
   docker-pull nginx
   ```

2. 拉取指定标签的镜像：
   ```bash
   docker-pull nginx:1.29.7
   ```

3. 使用 digest 精确拉取镜像：
   ```bash
   docker-pull ubuntu@sha256:12345abcdef...
   ```

4. 指定输出目录：
   ```bash
   docker-pull -o ./my-images nginx
   ```

5. 指定架构：
   ```bash
   docker-pull -a arm64 nginx
   ```

6. 使用代理：
   ```bash
   docker-pull -p http://proxy.example.com:8080 nginx
   ```

7. 使用自定义镜像仓库：
   ```bash
   docker-pull -m docker.1ms.run nginx
   ```

### 环境变量

工具会自动从以下环境变量中读取代理设置（如果命令行未指定）：
- `HTTP_PROXY`
- `HTTPS_PROXY`
- `http_proxy`
- `https_proxy`

### 配置文件

工具支持通过 YAML 配置文件（`config.yml`）来设置参数，配置文件的优先级低于命令行参数和环境变量。

#### 配置文件格式

```yaml
# 自定义镜像仓库地址
mirror: docker.1ms.run

# 代理 URL
proxy: http://proxy.example.com:8080

# 架构
arch: amd64

# 操作系统
os: linux
```

#### 配置文件优先级

1. 命令行参数（最高）
2. 环境变量（仅代理设置）
3. 配置文件
4. 默认值（最低）

#### 配置文件示例

创建 `config.yml` 文件：

```yaml
mirror: docker.1ms.run
proxy: http://proxy.example.com:8080
arch: amd64
os: linux
```

然后运行：

```bash
docker-pull nginx
```

工具会自动加载配置文件中的设置。

## 输出格式

工具会在指定的输出目录中生成一个 tar 包，文件名格式为：

```
<镜像名>_<标签>_<操作系统>_<架构>.tar
```

例如：
- `nginx_latest_linux_amd64.tar`
- `ubuntu_20.04_linux_arm64.tar`

## 加载镜像

生成的 tar 包可以使用 Docker 命令加载：

```bash
docker load -i <生成的tar包>
```

## 常见问题

### 下载失败
- 检查网络连接是否正常
- 检查代理设置是否正确
- 尝试增加重试次数（默认最大重试次数为 10）

### 镜像加载失败
- 确保生成的 tar 包完整且未损坏
- 检查 Docker 版本是否支持该镜像的架构

## 项目结构

```
docker-pull/
├── cmd/
│   └── docker-pull/        # 主命令入口
├── internal/
│   ├── archive/            # 打包功能
│   ├── config/             # 配置管理
│   ├── download/           # 下载功能
│   ├── registry/           # 镜像仓库交互
│   ├── ui/                 # 交互式界面（已移除）
│   └── verify/             # 校验功能
├── go.mod                  # Go 模块文件
├── go.sum                  # 依赖校验文件
└── README.md               # 项目说明
```

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request 来改进这个项目！