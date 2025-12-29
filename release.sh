#!/bin/bash

# Release 脚本 - 构建常用平台并发布到 GitHub Releases 和 Docker Hub

set -e

APP_NAME="ssl-manager"
OUTPUT_DIR="dist"
GITHUB_REPO="Davied-H/ssl-manager"
DOCKER_REPO="dongdonghe122/ssl-manager"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 常用平台（默认构建这些）
COMMON_PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
)

# 所有支持的平台
ALL_PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "linux/arm"
    "linux/386"
    "darwin/amd64"
    "darwin/arm64"
)

# Docker 多架构平台
DOCKER_PLATFORMS="linux/amd64,linux/arm64"

# 显示帮助信息
show_help() {
    echo -e "${BLUE}SSL Manager Release 脚本${NC}"
    echo ""
    echo "用法: $0 <版本号> [选项]"
    echo ""
    echo "参数:"
    echo "  版本号                   发布版本 (例如: 0.0.1, 1.0.0)"
    echo ""
    echo "选项:"
    echo "  -a, --all               构建所有二进制平台 (默认只构建常用平台)"
    echo "  -d, --draft             创建草稿 Release"
    echo "  -p, --prerelease        标记为预发布版本"
    echo "  --no-tag                不创建 git tag (假设 tag 已存在)"
    echo "  --no-push               不推送 tag 到远程"
    echo "  --no-docker             跳过 Docker 构建和推送"
    echo "  --docker-only           只构建和推送 Docker 镜像"
    echo "  --build-only            只构建，不发布 (包括二进制和 Docker)"
    echo "  -h, --help              显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  $0 0.0.1                发布 v0.0.1 (二进制 + Docker)"
    echo "  $0 1.0.0 -a             发布 v1.0.0 并构建所有平台"
    echo "  $0 0.1.0-beta -p        发布 v0.1.0-beta 预发布版本"
    echo "  $0 0.0.2 --no-docker    只发布二进制，跳过 Docker"
    echo "  $0 0.0.2 --docker-only  只构建和推送 Docker 镜像"
    echo ""
    echo "常用二进制平台 (默认):"
    for p in "${COMMON_PLATFORMS[@]}"; do
        echo "  - $p"
    done
    echo ""
    echo "Docker 多架构平台:"
    echo "  - ${DOCKER_PLATFORMS}"
    echo ""
    echo "Docker Hub: ${DOCKER_REPO}"
    echo "GitHub:     ${GITHUB_REPO}"
}

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查依赖
check_dependencies() {
    local check_docker=$1
    log_info "检查依赖..."

    if ! command -v go &> /dev/null; then
        log_error "go 未安装"
        exit 1
    fi

    if ! command -v git &> /dev/null; then
        log_error "git 未安装"
        exit 1
    fi

    if ! command -v gh &> /dev/null; then
        log_error "gh (GitHub CLI) 未安装"
        log_info "安装方法: brew install gh 或参考 https://cli.github.com/"
        exit 1
    fi

    # 检查 gh 是否已认证
    if ! gh auth status &> /dev/null; then
        log_error "gh 未认证，请先运行: gh auth login"
        exit 1
    fi

    # 检查 Docker（如果需要）
    if [ "$check_docker" = true ]; then
        if ! command -v docker &> /dev/null; then
            log_error "docker 未安装"
            exit 1
        fi

        # 检查 Docker 是否运行
        if ! docker info &> /dev/null; then
            log_error "Docker 未运行，请先启动 Docker"
            exit 1
        fi

        # 检查是否已登录 Docker Hub
        if ! docker info 2>/dev/null | grep -q "Username"; then
            log_warn "Docker Hub 可能未登录，推送时可能需要认证"
        fi

        # 检查 buildx 是否可用
        if ! docker buildx version &> /dev/null; then
            log_warn "docker buildx 不可用，将使用普通构建（仅当前平台）"
        fi
    fi

    log_success "依赖检查通过"
}

# 构建单个平台
build_platform() {
    local platform=$1
    local version=$2
    local os=${platform%/*}
    local arch=${platform#*/}
    local output="${OUTPUT_DIR}/${APP_NAME}-${os}-${arch}"

    # Windows 添加 .exe 后缀
    if [ "${os}" = "windows" ]; then
        output="${output}.exe"
    fi

    local build_time=$(date -u '+%Y-%m-%d %H:%M:%S')
    local git_commit=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    local ldflags="-s -w -X 'main.Version=${version}' -X 'main.BuildTime=${build_time}' -X 'main.GitCommit=${git_commit}'"

    # 特殊架构处理
    local extra_env=""
    case "${arch}" in
        arm)
            extra_env="GOARM=7"
            ;;
    esac

    printf "  %-20s" "${platform}"

    if env GOOS=${os} GOARCH=${arch} ${extra_env} go build -ldflags="${ldflags}" -o "${output}" ./cmd/ssl-manager 2>/dev/null; then
        local size=$(ls -lh "${output}" | awk '{print $5}')
        echo -e "${GREEN}✓${NC} ${size}"
        return 0
    else
        echo -e "${RED}✗ 构建失败${NC}"
        return 1
    fi
}

# 构建 Docker 镜像
build_docker() {
    local version=$1
    local build_time=$(date -u '+%Y-%m-%d %H:%M:%S')
    local git_commit=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

    echo ""
    log_info "构建 Docker 镜像..."

    # 检查是否支持 buildx 多架构构建
    if docker buildx version &> /dev/null; then
        log_info "使用 buildx 构建多架构镜像: ${DOCKER_PLATFORMS}"

        # 创建或使用 builder
        if ! docker buildx inspect multiarch-builder &> /dev/null; then
            log_info "创建 buildx builder..."
            docker buildx create --name multiarch-builder --use
        else
            docker buildx use multiarch-builder
        fi

        # 构建并推送多架构镜像
        docker buildx build \
            --platform "${DOCKER_PLATFORMS}" \
            --build-arg VERSION="${version}" \
            --build-arg BUILD_TIME="${build_time}" \
            --build-arg GIT_COMMIT="${git_commit}" \
            -t "${DOCKER_REPO}:${version}" \
            -t "${DOCKER_REPO}:latest" \
            --push \
            .

        log_success "Docker 多架构镜像构建并推送成功"
    else
        log_info "使用普通构建（仅当前平台）"

        # 普通构建
        docker build \
            --build-arg VERSION="${version}" \
            --build-arg BUILD_TIME="${build_time}" \
            --build-arg GIT_COMMIT="${git_commit}" \
            -t "${DOCKER_REPO}:${version}" \
            -t "${DOCKER_REPO}:latest" \
            .

        log_success "Docker 镜像构建成功"

        # 推送镜像
        log_info "推送 Docker 镜像到 Docker Hub..."
        docker push "${DOCKER_REPO}:${version}"
        docker push "${DOCKER_REPO}:latest"

        log_success "Docker 镜像推送成功"
    fi

    echo ""
    echo -e "  镜像: ${GREEN}${DOCKER_REPO}:${version}${NC}"
    echo -e "  镜像: ${GREEN}${DOCKER_REPO}:latest${NC}"
}

# 解析参数
VERSION=""
BUILD_ALL=false
IS_DRAFT=false
IS_PRERELEASE=false
CREATE_TAG=true
PUSH_TAG=true
BUILD_ONLY=false
SKIP_DOCKER=false
DOCKER_ONLY=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -a|--all)
            BUILD_ALL=true
            shift
            ;;
        -d|--draft)
            IS_DRAFT=true
            shift
            ;;
        -p|--prerelease)
            IS_PRERELEASE=true
            shift
            ;;
        --no-tag)
            CREATE_TAG=false
            shift
            ;;
        --no-push)
            PUSH_TAG=false
            shift
            ;;
        --no-docker)
            SKIP_DOCKER=true
            shift
            ;;
        --docker-only)
            DOCKER_ONLY=true
            shift
            ;;
        --build-only)
            BUILD_ONLY=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        -*)
            log_error "未知选项: $1"
            show_help
            exit 1
            ;;
        *)
            if [ -z "$VERSION" ]; then
                VERSION=$1
            else
                log_error "多余的参数: $1"
                show_help
                exit 1
            fi
            shift
            ;;
    esac
done

# 验证版本号
if [ -z "$VERSION" ]; then
    log_error "请指定版本号"
    echo ""
    show_help
    exit 1
fi

# 标准化版本号（确保有 v 前缀用于 tag）
TAG_VERSION="v${VERSION#v}"
DISPLAY_VERSION="${VERSION#v}"

echo ""
echo -e "${BLUE}========================================"
echo -e "  ${APP_NAME} Release 脚本"
echo -e "========================================${NC}"
echo ""
echo -e "版本:       ${GREEN}${DISPLAY_VERSION}${NC}"
echo -e "Tag:        ${GREEN}${TAG_VERSION}${NC}"
echo -e "GitHub:     ${GITHUB_REPO}"
echo -e "Docker Hub: ${DOCKER_REPO}"
echo -e "草稿:       $([ "$IS_DRAFT" = true ] && echo "是" || echo "否")"
echo -e "预发布:     $([ "$IS_PRERELEASE" = true ] && echo "是" || echo "否")"
echo -e "构建 Docker: $([ "$SKIP_DOCKER" = true ] && echo "否" || echo "是")"
echo ""

# 检查依赖
check_dependencies $([ "$SKIP_DOCKER" = true ] && echo false || echo true)

# 如果是 docker-only 模式，直接构建 Docker
if [ "$DOCKER_ONLY" = true ]; then
    build_docker "${DISPLAY_VERSION}"
    echo ""
    log_success "Docker 镜像发布完成!"
    echo ""
    echo -e "Docker Hub: ${BLUE}https://hub.docker.com/r/${DOCKER_REPO}${NC}"
    exit 0
fi

# 确定构建平台
if [ "$BUILD_ALL" = true ]; then
    BUILD_PLATFORMS=("${ALL_PLATFORMS[@]}")
    log_info "将构建所有平台 (${#BUILD_PLATFORMS[@]} 个)"
else
    BUILD_PLATFORMS=("${COMMON_PLATFORMS[@]}")
    log_info "将构建常用平台 (${#BUILD_PLATFORMS[@]} 个)"
fi

# 检查工作目录是否干净
if [ "$CREATE_TAG" = true ]; then
    if ! git diff --quiet HEAD 2>/dev/null; then
        log_warn "工作目录有未提交的更改"
        read -p "是否继续? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "已取消"
            exit 1
        fi
    fi
fi

# 清理并创建输出目录
log_info "清理构建目录..."
rm -rf ${OUTPUT_DIR}
mkdir -p ${OUTPUT_DIR}

# 构建二进制文件
echo ""
log_info "开始构建二进制文件..."
echo ""

success_count=0
fail_count=0

for platform in "${BUILD_PLATFORMS[@]}"; do
    if build_platform "${platform}" "${DISPLAY_VERSION}"; then
        ((success_count++))
    else
        ((fail_count++))
    fi
done

echo ""
log_info "二进制构建完成: ${GREEN}${success_count} 成功${NC}, ${RED}${fail_count} 失败${NC}"

if [ ${fail_count} -gt 0 ]; then
    log_error "部分平台构建失败"
    exit 1
fi

# 生成校验和
log_info "生成 SHA256 校验和..."
if command -v sha256sum &> /dev/null; then
    (cd ${OUTPUT_DIR} && sha256sum ssl-manager-* > checksums.sha256)
elif command -v shasum &> /dev/null; then
    (cd ${OUTPUT_DIR} && shasum -a 256 ssl-manager-* > checksums.sha256)
fi
log_success "校验和文件: ${OUTPUT_DIR}/checksums.sha256"

# 显示构建产物
echo ""
log_info "构建产物:"
ls -lh ${OUTPUT_DIR}/

# 构建 Docker 镜像
if [ "$SKIP_DOCKER" = false ]; then
    build_docker "${DISPLAY_VERSION}"
fi

# 如果只构建则退出
if [ "$BUILD_ONLY" = true ]; then
    echo ""
    log_success "构建完成 (仅构建模式)"
    exit 0
fi

# 创建 tag
if [ "$CREATE_TAG" = true ]; then
    echo ""
    log_info "创建 Git Tag: ${TAG_VERSION}"

    # 检查 tag 是否已存在
    if git rev-parse "${TAG_VERSION}" &> /dev/null; then
        log_warn "Tag ${TAG_VERSION} 已存在"
        read -p "是否删除并重新创建? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            git tag -d "${TAG_VERSION}"
            if [ "$PUSH_TAG" = true ]; then
                git push origin ":refs/tags/${TAG_VERSION}" 2>/dev/null || true
            fi
        else
            log_info "使用已存在的 tag"
            CREATE_TAG=false
        fi
    fi

    if [ "$CREATE_TAG" = true ]; then
        git tag -a "${TAG_VERSION}" -m "Release ${TAG_VERSION}"
        log_success "Tag 创建成功"
    fi
fi

# 推送 tag
if [ "$PUSH_TAG" = true ] && [ "$CREATE_TAG" = true ]; then
    log_info "推送 Tag 到远程..."
    git push origin "${TAG_VERSION}"
    log_success "Tag 推送成功"
fi

# 生成 Release Notes
RELEASE_NOTES="## SSL Manager ${TAG_VERSION}

### 下载

#### 二进制文件

| 平台 | 架构 | 下载 |
|-----|------|------|"

for platform in "${BUILD_PLATFORMS[@]}"; do
    os=${platform%/*}
    arch=${platform#*/}
    filename="${APP_NAME}-${os}-${arch}"
    [ "${os}" = "windows" ] && filename="${filename}.exe"

    case "${os}" in
        linux)   os_display="Linux" ;;
        darwin)  os_display="macOS" ;;
        windows) os_display="Windows" ;;
        *)       os_display="${os}" ;;
    esac

    case "${arch}" in
        amd64) arch_display="x86_64" ;;
        arm64) arch_display="ARM64" ;;
        arm)   arch_display="ARMv7" ;;
        386)   arch_display="x86" ;;
        *)     arch_display="${arch}" ;;
    esac

    RELEASE_NOTES="${RELEASE_NOTES}
| ${os_display} | ${arch_display} | [${filename}](https://github.com/${GITHUB_REPO}/releases/download/${TAG_VERSION}/${filename}) |"
done

# 添加 Docker 信息
if [ "$SKIP_DOCKER" = false ]; then
    RELEASE_NOTES="${RELEASE_NOTES}

#### Docker 镜像

\`\`\`bash
# 拉取最新版本
docker pull ${DOCKER_REPO}:latest

# 拉取指定版本
docker pull ${DOCKER_REPO}:${DISPLAY_VERSION}
\`\`\`

支持架构: \`linux/amd64\`, \`linux/arm64\`"
fi

RELEASE_NOTES="${RELEASE_NOTES}

### 校验和

下载 \`checksums.sha256\` 文件验证下载完整性。

### 安装

#### 二进制安装

\`\`\`bash
# Linux/macOS
chmod +x ssl-manager-*
sudo mv ssl-manager-* /usr/local/bin/ssl-manager

# 验证安装
ssl-manager -h
\`\`\`

#### Docker 运行

\`\`\`bash
# 单次执行
docker run -v \$(pwd)/config.yaml:/app/config.yaml \\
           -v \$(pwd)/certs:/app/certs \\
           ${DOCKER_REPO}

# 守护进程模式
docker run -d \\
           -e SSL_MANAGER_MODE=daemon \\
           -v \$(pwd)/config.yaml:/app/config.yaml \\
           -v \$(pwd)/certs:/app/certs \\
           --name ssl-manager \\
           --restart unless-stopped \\
           ${DOCKER_REPO}
\`\`\`
"

# 创建 GitHub Release
echo ""
log_info "创建 GitHub Release..."

GH_ARGS=(
    "${TAG_VERSION}"
    --repo "${GITHUB_REPO}"
    --title "SSL Manager ${TAG_VERSION}"
    --notes "${RELEASE_NOTES}"
)

# 添加所有构建产物
for file in ${OUTPUT_DIR}/*; do
    GH_ARGS+=("${file}")
done

if [ "$IS_DRAFT" = true ]; then
    GH_ARGS+=("--draft")
fi

if [ "$IS_PRERELEASE" = true ]; then
    GH_ARGS+=("--prerelease")
fi

gh release create "${GH_ARGS[@]}"

echo ""
log_success "Release 发布成功!"
echo ""
echo -e "GitHub Release: ${BLUE}https://github.com/${GITHUB_REPO}/releases/tag/${TAG_VERSION}${NC}"
if [ "$SKIP_DOCKER" = false ]; then
    echo -e "Docker Hub:     ${BLUE}https://hub.docker.com/r/${DOCKER_REPO}${NC}"
fi
echo ""
