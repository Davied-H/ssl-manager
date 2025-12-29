#!/bin/bash

# 构建脚本 - 支持多平台多架构构建

set -e

APP_NAME="ssl-manager"
OUTPUT_DIR="dist"
VERSION=${VERSION:-"dev"}
BUILD_TIME=$(date -u '+%Y-%m-%d %H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 编译参数
LDFLAGS="-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}' -X 'main.GitCommit=${GIT_COMMIT}'"

# 支持的平台和架构
# 格式: "操作系统/架构"
PLATFORMS=(
    # Linux
    "linux/amd64"
    "linux/arm64"
    "linux/arm"
    "linux/386"
    "linux/mips"
    "linux/mipsle"
    "linux/mips64"
    "linux/mips64le"
    "linux/riscv64"
    "linux/loong64"
    # macOS
    "darwin/amd64"
    "darwin/arm64"
    # Windows
    "windows/amd64"
    "windows/arm64"
    "windows/386"
    # FreeBSD
    "freebsd/amd64"
    "freebsd/arm64"
    "freebsd/386"
    # OpenBSD
    "openbsd/amd64"
    "openbsd/arm64"
    # NetBSD
    "netbsd/amd64"
    "netbsd/arm64"
)

# 显示帮助信息
show_help() {
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -p, --platform PLATFORM  只构建指定平台 (例如: linux/amd64)"
    echo "  -a, --all                构建所有支持的平台 (默认)"
    echo "  -c, --current            只构建当前平台"
    echo "  -l, --list               列出所有支持的平台"
    echo "  -h, --help               显示此帮助信息"
    echo ""
    echo "环境变量:"
    echo "  VERSION                  版本号 (默认: dev)"
    echo ""
    echo "示例:"
    echo "  $0                       构建所有平台"
    echo "  $0 -c                    只构建当前平台"
    echo "  $0 -p linux/amd64        只构建 Linux amd64"
    echo "  $0 -p linux/arm64 -p darwin/amd64  构建多个指定平台"
}

# 列出所有支持的平台
list_platforms() {
    echo "支持的平台:"
    for platform in "${PLATFORMS[@]}"; do
        echo "  ${platform}"
    done
}

# 构建单个平台
build_platform() {
    local platform=$1
    local os=${platform%/*}
    local arch=${platform#*/}
    local output="${OUTPUT_DIR}/${APP_NAME}-${os}-${arch}"

    # Windows 添加 .exe 后缀
    if [ "${os}" = "windows" ]; then
        output="${output}.exe"
    fi

    echo "构建 ${platform}..."

    # 特殊架构处理
    local extra_env=""
    case "${arch}" in
        arm)
            extra_env="GOARM=7"
            ;;
        mips|mipsle)
            extra_env="GOMIPS=softfloat"
            ;;
    esac

    if env GOOS=${os} GOARCH=${arch} ${extra_env} go build -ldflags="${LDFLAGS}" -o "${output}" ./cmd/ssl-manager 2>/dev/null; then
        echo "  ✓ ${output}"
        return 0
    else
        echo "  ✗ ${platform} 构建失败"
        return 1
    fi
}

# 解析命令行参数
BUILD_PLATFORMS=()
PARSE_MODE="all"

while [[ $# -gt 0 ]]; do
    case $1 in
        -p|--platform)
            PARSE_MODE="custom"
            BUILD_PLATFORMS+=("$2")
            shift 2
            ;;
        -a|--all)
            PARSE_MODE="all"
            shift
            ;;
        -c|--current)
            PARSE_MODE="current"
            shift
            ;;
        -l|--list)
            list_platforms
            exit 0
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
done

# 确定要构建的平台列表
case "${PARSE_MODE}" in
    all)
        BUILD_PLATFORMS=("${PLATFORMS[@]}")
        ;;
    current)
        current_os=$(go env GOOS)
        current_arch=$(go env GOARCH)
        BUILD_PLATFORMS=("${current_os}/${current_arch}")
        ;;
    custom)
        # 验证自定义平台
        for platform in "${BUILD_PLATFORMS[@]}"; do
            valid=false
            for supported in "${PLATFORMS[@]}"; do
                if [ "${platform}" = "${supported}" ]; then
                    valid=true
                    break
                fi
            done
            if [ "${valid}" = false ]; then
                echo "警告: 平台 ${platform} 不在预定义列表中，将尝试构建"
            fi
        done
        ;;
esac

# 清理旧的构建目录
rm -rf ${OUTPUT_DIR}
mkdir -p ${OUTPUT_DIR}

echo "========================================"
echo "  ${APP_NAME} 构建脚本"
echo "========================================"
echo "版本: ${VERSION}"
echo "Git: ${GIT_COMMIT}"
echo "构建时间: ${BUILD_TIME}"
echo "目标平台数: ${#BUILD_PLATFORMS[@]}"
echo "========================================"
echo ""

# 执行构建
success_count=0
fail_count=0
failed_platforms=()

for platform in "${BUILD_PLATFORMS[@]}"; do
    if build_platform "${platform}"; then
        ((success_count++))
    else
        ((fail_count++))
        failed_platforms+=("${platform}")
    fi
done

echo ""
echo "========================================"
echo "  构建完成"
echo "========================================"
echo "成功: ${success_count}"
echo "失败: ${fail_count}"

if [ ${fail_count} -gt 0 ]; then
    echo ""
    echo "失败的平台:"
    for platform in "${failed_platforms[@]}"; do
        echo "  - ${platform}"
    done
fi

echo ""
echo "输出文件:"
ls -lh ${OUTPUT_DIR}/

# 生成校验和
if command -v sha256sum &> /dev/null; then
    echo ""
    echo "生成 SHA256 校验和..."
    (cd ${OUTPUT_DIR} && sha256sum * > checksums.sha256)
    echo "校验和文件: ${OUTPUT_DIR}/checksums.sha256"
elif command -v shasum &> /dev/null; then
    echo ""
    echo "生成 SHA256 校验和..."
    (cd ${OUTPUT_DIR} && shasum -a 256 * > checksums.sha256)
    echo "校验和文件: ${OUTPUT_DIR}/checksums.sha256"
fi

# 如果有失败则返回非零退出码
if [ ${fail_count} -gt 0 ]; then
    exit 1
fi