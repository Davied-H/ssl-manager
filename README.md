# SSL Manager

SSL 证书自动管理工具，支持自动申请、续期和下载 SSL 证书。

## 功能特性

- 支持多云平台：阿里云、腾讯云、华为云
- 支持混合模式：证书申请和 DNS 验证可使用不同云平台
- 自动检查域名证书过期时间
- 自动申请免费 SSL 证书
- 自动完成 DNS 验证
- 自动下载证书到本地指定目录
- 支持证书下载后执行自定义命令（如重载 Nginx）
- 支持守护进程模式持续监控
- 支持多域名批量管理

## 支持的云平台

| 云平台 | 证书申请 | DNS 验证 | 说明 |
|--------|---------|---------|------|
| 阿里云 | ✅ | ✅ | DigiCert 免费 DV 证书 |
| 腾讯云 | ✅ | ✅ | TrustAsia 免费 DV 证书 |
| 华为云 | ❌ | ✅ | 仅支持管理已有证书，不支持 API 申请 |

## 前置条件

### 阿里云

1. 域名已托管在阿里云 DNS
2. 拥有阿里云 AccessKey（需要 SSL 证书服务和 DNS 服务权限）
3. 阿里云账户有可用的免费证书额度

### 腾讯云

1. 域名已托管在 DNSPod
2. 拥有腾讯云 SecretId/SecretKey（需要 SSL 证书服务和 DNSPod 服务权限）
3. 腾讯云账户有可用的免费证书额度

### 华为云

1. 域名已托管在华为云 DNS
2. 拥有华为云 AccessKey（需要 DNS 服务权限）
3. **注意**：华为云不支持通过 API 申请免费证书，需在控制台手动申请后使用本工具管理

## 安装

### 下载预编译版本

从 [Releases](https://github.com/your-username/ssl-manager/releases) 页面下载适合您平台的预编译版本。

支持的平台：

| 操作系统 | 架构 | 文件名 |
|---------|------|--------|
| Linux | x86_64 (amd64) | `ssl-manager-linux-amd64` |
| Linux | ARM64 | `ssl-manager-linux-arm64` |
| Linux | ARMv7 | `ssl-manager-linux-arm` |
| Linux | x86 (386) | `ssl-manager-linux-386` |
| Linux | MIPS | `ssl-manager-linux-mips` |
| Linux | MIPS (小端) | `ssl-manager-linux-mipsle` |
| Linux | RISC-V 64 | `ssl-manager-linux-riscv64` |
| Linux | LoongArch 64 | `ssl-manager-linux-loong64` |
| macOS | Intel (amd64) | `ssl-manager-darwin-amd64` |
| macOS | Apple Silicon (arm64) | `ssl-manager-darwin-arm64` |
| Windows | x86_64 (amd64) | `ssl-manager-windows-amd64.exe` |
| Windows | ARM64 | `ssl-manager-windows-arm64.exe` |
| Windows | x86 (386) | `ssl-manager-windows-386.exe` |
| FreeBSD | amd64/arm64/386 | `ssl-manager-freebsd-*` |
| OpenBSD | amd64/arm64 | `ssl-manager-openbsd-*` |
| NetBSD | amd64/arm64 | `ssl-manager-netbsd-*` |

### 从源码构建

需要 Go 1.21 或更高版本。

```bash
# 克隆项目
git clone https://github.com/your-username/ssl-manager.git
cd ssl-manager

# 快速构建（当前平台）
go build -o ssl-manager ./cmd/ssl-manager

# 或使用构建脚本
./build.sh
```

### 构建脚本使用

构建脚本支持多种选项，可以灵活构建不同平台的二进制文件：

```bash
# 构建所有支持的平台（默认）
./build.sh

# 只构建当前平台（最快）
./build.sh -c
./build.sh --current

# 构建指定平台
./build.sh -p linux/amd64
./build.sh --platform linux/arm64

# 构建多个指定平台
./build.sh -p linux/amd64 -p linux/arm64 -p darwin/arm64

# 列出所有支持的平台
./build.sh -l
./build.sh --list

# 查看帮助
./build.sh -h
./build.sh --help

# 指定版本号构建
VERSION=1.0.0 ./build.sh
```

构建产物位于 `dist/` 目录，同时会生成 `checksums.sha256` 校验和文件。

## 配置

复制示例配置文件并修改：

```bash
cp config.yaml.example config.yaml
```

编辑 `config.yaml`：

```yaml
# 云平台凭证配置（按需配置，支持多云混用）
providers:
  # 阿里云配置
  aliyun:
    access_key_id: "your_access_key_id"
    access_key_secret: "your_access_key_secret"
    region: "cn-hangzhou"

  # 腾讯云配置
  tencent:
    secret_id: "your_secret_id"
    secret_key: "your_secret_key"
    region: "ap-guangzhou"

  # 华为云配置
  # huawei:
  #   access_key: "your_access_key"
  #   secret_key: "your_secret_key"
  #   region: "cn-east-2"
  #   project_id: "your_project_id"

# 域名配置
domains:
  # 简单模式：证书和DNS使用同一平台
  - domain: "example.com"
    provider: "aliyun"        # 使用阿里云
    renew_days: 7             # 证书到期前多少天开始续期
    # post_command: "nginx -s reload"  # 可选，域名级别的后置命令

  - domain: "www.example.com"
    provider: "tencent"       # 使用腾讯云
    renew_days: 7

  # 混合模式：证书和DNS使用不同平台
  # - domain: "api.example.com"
  #   cert_provider: "tencent"  # 证书提供商
  #   dns_provider: "aliyun"    # DNS 验证提供商
  #   renew_days: 7

# 证书下载目录
output_dir: "./certs"

# 检查间隔（小时），守护进程模式使用
check_interval: 24

# 全局的证书下载后执行的命令（可选）
# 支持的变量:
#   ${DOMAIN}         - 域名
#   ${CERT_DIR}       - 证书目录
#   ${CERT_FILE}      - 证书文件路径
#   ${KEY_FILE}       - 私钥文件路径
#   ${FULLCHAIN_FILE} - 完整证书链文件路径
# post_command: "systemctl reload nginx"
```

## 使用方法

### 单次检查并申请证书

```bash
./ssl-manager config.yaml
```

### 守护进程管理

```bash
# 启动后台守护进程
./ssl-manager config.yaml start

# 停止守护进程
./ssl-manager config.yaml stop

# 重启守护进程
./ssl-manager config.yaml restart

# 查看运行状态
./ssl-manager config.yaml status
```

### 前台守护进程模式（用于调试）

```bash
./ssl-manager config.yaml daemon
```

### 继续处理已有订单

如果之前的申请中断，可以继续处理：

```bash
./ssl-manager config.yaml continue <订单ID> <域名> [提供商]
```

示例：

```bash
# 阿里云订单
./ssl-manager config.yaml continue 123456789 example.com aliyun

# 腾讯云订单
./ssl-manager config.yaml continue abcd1234 example.com tencent
```

### 查看帮助

```bash
./ssl-manager -h
```

## Docker 部署

### 构建镜像

```bash
docker build -t ssl-manager .
```

### 运行容器

```bash
# 单次执行模式
docker run -v $(pwd)/config.yaml:/app/config.yaml \
           -v $(pwd)/certs:/app/certs \
           ssl-manager

# 守护进程模式
docker run -d \
           -e SSL_MANAGER_MODE=daemon \
           -v $(pwd)/config.yaml:/app/config.yaml \
           -v $(pwd)/certs:/app/certs \
           --name ssl-manager \
           ssl-manager
```

### Docker Compose

```yaml
version: '3'
services:
  ssl-manager:
    build: .
    environment:
      - SSL_MANAGER_MODE=daemon
      - TZ=Asia/Shanghai
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./certs:/app/certs
    restart: unless-stopped
```

## 证书文件

证书下载后保存在 `output_dir/<域名>/` 目录下：

- `cert.pem` - 证书文件
- `key.pem` - 私钥文件
- `fullchain.pem` - 完整证书链

## Nginx 配置示例

```nginx
server {
    listen 443 ssl;
    server_name example.com;

    ssl_certificate /path/to/certs/example.com/fullchain.pem;
    ssl_certificate_key /path/to/certs/example.com/key.pem;

    # 其他配置...
}
```

## 自动续期（配合 Cron）

添加 crontab 任务，每天检查一次：

```bash
# 每天凌晨 3 点检查证书
0 3 * * * /path/to/ssl-manager /path/to/config.yaml >> /var/log/ssl-manager.log 2>&1
```

## 注意事项

- 阿里云和腾讯云免费证书每年有申请数量限制
- 华为云不支持通过 API 申请免费证书，仅可管理已有证书
- DNS 验证记录会自动添加和更新，验证完成后请手动清理
- 建议设置 `renew_days` 为 7-14 天，预留足够的续期时间
- `config.yaml` 包含敏感的 AccessKey 信息，请妥善保管

## 许可证

MIT License
