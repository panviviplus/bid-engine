#!/usr/bin/env bash
#
# ===============================================================
# 标擎 Docker 一键部署脚本
#
# 用法:
#
# 1. 部署前后端（推荐）
#      bash docker-start.sh <TAG>
#          -> 前后端均使用该 TAG
#
#      bash docker-start.sh <BACKEND_TAG> <FRONTEND_TAG>
#          -> 后端、前端分别使用不同 TAG
#
# 2. 仅部署后端
#      bash docker-start.sh backend <TAG>
#      bash docker-start.sh be <TAG>
#
# 3. 仅部署前端
#      bash docker-start.sh frontend <TAG>
#      bash docker-start.sh fe <TAG>
#
# 示例:
#
#   docker build -t bid-engine-backend:v7 \
#       -f bid-engine-backend/Dockerfile bid-engine-backend/
#
#   docker build -t bid-engine-frontend:v7 \
#       -f bid-engine-frontend/Dockerfile bid-engine-frontend/
#
#   bash docker-start.sh v7
#   bash docker-start.sh v7 v8
#   bash docker-start.sh backend v7
#   bash docker-start.sh frontend v7
#
# 注意:
#   - Redis / MinIO / Docling / 招标情报采集服务（tender-intel-collector）等基础服务需已启动
#   - MySQL 运行在宿主机
#   - 必须显式指定镜像 TAG，不会默认使用 latest
# ===============================================================

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ -f "$PROJECT_DIR/.env.local" ]; then
    set -a
    # shellcheck disable=SC1091
    source "$PROJECT_DIR/.env.local"
    set +a
fi

usage() {
cat <<EOF
==============================================================
标擎 Docker 部署

用法：

  部署前后端：
    bash docker-start.sh <TAG>
    bash docker-start.sh <BACKEND_TAG> <FRONTEND_TAG>

  仅部署后端：
    bash docker-start.sh backend <TAG>
    bash docker-start.sh be <TAG>

  仅部署前端：
    bash docker-start.sh frontend <TAG>
    bash docker-start.sh fe <TAG>

示例：

    bash docker-start.sh v7
    bash docker-start.sh v7 v8
    bash docker-start.sh backend v7
    bash docker-start.sh frontend v7

==============================================================
EOF
}

# 未传参数，打印帮助
if [[ $# -eq 0 ]]; then
    usage
    exit 1
fi

MODE="all"
BACKEND_TAG=""
FRONTEND_TAG=""

case "$1" in
    backend|be)
        if [[ $# -lt 2 ]]; then
            echo "错误：请指定后端镜像 TAG。"
            echo
            usage
            exit 1
        fi
        MODE="backend"
        BACKEND_TAG="$2"
        ;;

    frontend|fe)
        if [[ $# -lt 2 ]]; then
            echo "错误：请指定前端镜像 TAG。"
            echo
            usage
            exit 1
        fi
        MODE="frontend"
        FRONTEND_TAG="$2"
        ;;

    *)
        MODE="all"
        BACKEND_TAG="$1"
        FRONTEND_TAG="${2:-$BACKEND_TAG}"
        ;;
esac

export BACKEND_TAG
export FRONTEND_TAG

echo "=============================================="
echo "  标擎 Docker 部署"
echo "=============================================="

case "$MODE" in
    all)
        echo "模式      : 前后端"
        echo "后端镜像  : bid-engine-backend:${BACKEND_TAG}"
        echo "前端镜像  : bid-engine-frontend:${FRONTEND_TAG}"
        echo

        echo "→ 重建后端容器..."
        docker compose -f docker-compose-backend.yaml up -d

        echo

        echo "→ 重建前端容器..."
        docker compose -f docker-compose-frontend.yaml up -d
        ;;

    backend)
        echo "模式      : 仅后端"
        echo "后端镜像  : bid-engine-backend:${BACKEND_TAG}"
        echo

        echo "→ 重建后端容器..."
        docker compose -f docker-compose-backend.yaml up -d
        ;;

    frontend)
        echo "模式      : 仅前端"
        echo "前端镜像  : bid-engine-frontend:${FRONTEND_TAG}"
        echo

        echo "→ 重建前端容器..."
        docker compose -f docker-compose-frontend.yaml up -d
        ;;
esac

echo
echo "✓ 部署完成"
echo
echo "访问地址:"
echo "  后端: http://localhost:8081"
echo "  前端: http://localhost:8080"
echo
echo "日志查看:"
echo "  docker compose -f docker-compose-backend.yaml logs -f"
echo "  docker compose -f docker-compose-frontend.yaml logs -f"
echo "  docker compose -f docker-compose.yml logs -f"
