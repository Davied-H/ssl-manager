# ============================================
# SSL Manager Docker Image
# 多阶段构建，支持单次执行和守护进程模式
# ============================================

# 阶段1: 构建阶段
FROM golang:1.24-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /build

# 复制 go.mod 和 go.sum，利用 Docker 缓存
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建参数
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT=unknown

# 编译静态二进制文件
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}' -X 'main.GitCommit=${GIT_COMMIT}'" \
    -o ssl-manager \
    ./cmd/ssl-manager

# ============================================
# 阶段2: 运行阶段
# ============================================
FROM alpine:3.19

# 安装运行时依赖
# - ca-certificates: HTTPS 请求需要
# - tzdata: 时区支持（证书检查需要正确时间）
RUN apk add --no-cache ca-certificates tzdata

# 设置时区（默认亚洲/上海，可通过环境变量覆盖）
ENV TZ=Asia/Shanghai

# 创建非 root 用户
RUN addgroup -S sslmanager && adduser -S sslmanager -G sslmanager

# 创建必要目录
RUN mkdir -p /app/certs /app/config && \
    chown -R sslmanager:sslmanager /app

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /build/ssl-manager /app/ssl-manager

# 复制示例配置（方便用户参考）
COPY --from=builder /build/config.yaml.example /app/config/config.yaml.example

# 复制入口点脚本
COPY entrypoint.sh /app/entrypoint.sh

# 设置文件权限
RUN chmod +x /app/ssl-manager /app/entrypoint.sh && \
    chown sslmanager:sslmanager /app/ssl-manager /app/entrypoint.sh

# 切换到非 root 用户
USER sslmanager

# 定义数据卷
VOLUME ["/app/config", "/app/certs"]

# 环境变量
# SSL_MANAGER_MODE: once (单次执行) 或 daemon (守护进程模式)
ENV SSL_MANAGER_MODE=once

# 健康检查（守护进程模式下检查进程是否存在）
HEALTHCHECK --interval=60s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep ssl-manager > /dev/null || exit 1

# 入口点
ENTRYPOINT ["/app/entrypoint.sh"]
