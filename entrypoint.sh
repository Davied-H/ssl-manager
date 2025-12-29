#!/bin/sh
set -e

CONFIG_FILE="/app/config/config.yaml"

# 检查配置文件是否存在
if [ ! -f "$CONFIG_FILE" ]; then
    echo "============================================"
    echo "错误: 配置文件不存在: $CONFIG_FILE"
    echo "============================================"
    echo ""
    echo "请挂载配置文件到 /app/config/config.yaml"
    echo ""
    echo "示例:"
    echo "  docker run --rm \\"
    echo "    -v \$(pwd)/config.yaml:/app/config/config.yaml:ro \\"
    echo "    -v \$(pwd)/certs:/app/certs \\"
    echo "    ssl-manager:latest"
    echo ""
    echo "参考示例配置: /app/config/config.yaml.example"
    exit 1
fi

# 根据模式运行
case "$SSL_MANAGER_MODE" in
    daemon)
        echo "[SSL Manager] 启动守护进程模式..."
        echo "[SSL Manager] 配置文件: $CONFIG_FILE"
        exec /app/ssl-manager "$CONFIG_FILE" daemon
        ;;
    once|*)
        echo "[SSL Manager] 启动单次执行模式..."
        echo "[SSL Manager] 配置文件: $CONFIG_FILE"
        exec /app/ssl-manager "$CONFIG_FILE"
        ;;
esac
