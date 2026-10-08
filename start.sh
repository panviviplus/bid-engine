#!/usr/bin/env bash
#
# ===============================================================
#   标擎（智能投标助手） — 本地前后端一键启动
#   （职责单一：不管理任何容器，只启动本地前后端服务）
# ===============================================================
#
# 用法:
#   bash start.sh          # 启动所有本地服务
#   bash start.sh --help   # 查看帮助
#
# 职责:
#   1. 释放宿主机的 3000 / 1022 端口（被占用则 kill）
#   2. 启动 Go 后端 (端口 1022)
#   3. 启动 Next.js 前端 (端口 3000)
#   4. 打开浏览器 http://localhost:3000/signin
#
# 本脚本【不做】的事:
#   - 不启动 / 停止 / 重建 Redis、MinIO、Docling、招标情报采集服务、doc-converter 容器（只检查并提醒）
#   - 不启动 MySQL（只检查并提醒）
#   - 不做任何镜像构建、容器更新（容器部署见 docker-start.sh）
# ===============================================================

set -euo pipefail

# ---------- 帮助 ----------
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    echo "用法: bash start.sh"
    echo ""
    echo "本地前后端一键启动（不管理任何容器）:"
    echo "  1. 环境预检（只读检查 Go/Node/MySQL/基础容器状态）"
    echo "  2. 释放宿主机 3000 / 1022 端口"
    echo "  3. 配置前端环境变量（指向本地后端 localhost:1022）"
    echo "  4. 启动 Go 后端 (端口 1022，配置 conf/conf-local.yml)"
    echo "  5. 启动 Next.js 前端 (端口 3000)"
    echo "  6. 打开浏览器 http://localhost:3000/signin"
    echo ""
    echo "前置依赖（脚本只检查不启动）:"
    echo "  - MySQL 运行在 127.0.0.1:3306 (root/root)"
    echo "  - 基础容器 redis/minio/docling/doc-converter/tender-intel-collector 已运行（未运行则脚本仅提醒）"
    echo "    手动启动: docker compose -f docker-compose.yml up -d"
    exit 0
fi

# ---------- 项目路径 ----------
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKEND_DIR="$PROJECT_DIR/bid-engine-backend"
FRONTEND_DIR="$PROJECT_DIR/bid-engine-frontend"

if [ -f "$PROJECT_DIR/.env.local" ]; then
    set -a
    # shellcheck disable=SC1091
    source "$PROJECT_DIR/.env.local"
    set +a
fi

BACKEND_PID=""
FRONTEND_PID=""
FRONTEND_PORT=3000
BACKEND_PORT=1022

# ---------- 颜色输出 ----------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

print_banner() {
    echo ""
    echo -e "${CYAN}${BOLD}===============================================${NC}"
    echo -e "${CYAN}${BOLD}    标擎（智能投标助手） — 本地前后端启动${NC}"
    echo -e "${CYAN}${BOLD}===============================================${NC}"
    echo ""
}

print_step() {
    echo -e "${BLUE}${BOLD}[$1/$2]${NC} $3"
}

print_ok() {
    echo -e "  ${GREEN}✓${NC} $1"
}

print_warn() {
    echo -e "  ${YELLOW}⚠${NC} $1"
}

print_error() {
    echo -e "  ${RED}✗${NC} $1"
}

# ---------- OS 检测 ----------
detect_os() {
    case "$(uname -s)" in
        Darwin*)  echo "macos" ;;
        MINGW*|MSYS*) echo "windows" ;;
        Linux*)
            if grep -qi microsoft /proc/version 2>/dev/null; then
                echo "wsl"
            else
                echo "linux"
            fi
            ;;
        *) echo "unknown" ;;
    esac
}

OS=$(detect_os)

# ---------- 释放被占用的端口 ----------
kill_port() {
    local port=$1
    local pids
    pids=$(lsof -ti:"$port" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        echo "  → 端口 $port 被占用，正在释放..."
        echo "$pids" | xargs kill -9 2>/dev/null || true
        sleep 0.5
        print_ok "端口 $port 已释放"
    fi
}

TOTAL_STEPS=6

# =============================================================
#  STEP 1 — 环境预检（只读，不做任何启动/修改操作）
# =============================================================
preflight_check() {
    print_step 1 $TOTAL_STEPS "环境预检"

    # Go
    if command -v go &>/dev/null; then
        print_ok "Go $(go version | awk '{print $3}')"
    else
        print_error "未安装 Go (>= 1.23)，请先安装"
        return 1
    fi

    # Node
    if command -v node &>/dev/null; then
        print_ok "Node.js $(node -v)"
    else
        print_error "未安装 Node.js (>= 22)，请先安装"
        return 1
    fi

    # MySQL：与后端 conf-local.yml 的 DSN 完全一致（127.0.0.1:3306 root/root）
    if mysql -h 127.0.0.1 -P 3306 -u root -proot -e "SELECT 1" &>/dev/null; then
        print_ok "MySQL 已就绪 (127.0.0.1:3306, root/root)"
    else
        print_error "MySQL 不可达或账号不对 (127.0.0.1:3306 root/root)"
        print_error "请先启动 MySQL 并确认密码为 root，例如: brew services start mysql"
        return 1
    fi

    # 基础容器（Redis / MinIO / Docling）：只查看状态，不启动、不操作
    local docker_ok=false
    if docker ps --format '{{.Names}}' &>/dev/null; then
        docker_ok=true
    fi
    if [ "$docker_ok" = false ]; then
        print_warn "Docker daemon 未运行，无法检查基础容器状态"
        print_warn "Redis / MinIO / Docling / 招标情报采集服务 / Doc-Converter 未运行时，相关功能将不可用（脚本不负责启动）"
        print_warn "手动启动: docker compose -f docker-compose.yml up -d"
    else
        local missing=""
        local names
        names=$(docker ps --format '{{.Names}}')
        for c in redis minio docling doc-converter tender-intel-collector; do
            if ! echo "$names" | grep -qx "$c"; then
                missing="$missing $c"
            fi
        done
        if [ -n "$missing" ]; then
            print_warn "基础容器未运行:${missing}（仅提醒，脚本不启动）"
            print_warn "如需启动: docker compose -f docker-compose.yml up -d"
        else
            print_ok "基础容器 Redis / MinIO / Docling / 招标情报采集服务 / Doc-Converter 均在运行（脚本不管理，保持现状）"
        fi
    fi

    return 0
}

# =============================================================
#  STEP 2 — 释放端口
# =============================================================
free_ports() {
    print_step 2 $TOTAL_STEPS "释放端口 $FRONTEND_PORT / $BACKEND_PORT"

    kill_port "$FRONTEND_PORT"
    kill_port "$BACKEND_PORT"
}

# =============================================================
#  STEP 3 — 配置前端环境变量（指向本地后端）
# =============================================================
setup_frontend_env() {
    print_step 3 $TOTAL_STEPS "配置前端环境变量"

    local env_file="$FRONTEND_DIR/.env.development.local"
    cat > "$env_file" <<EOF
NEXT_PUBLIC_USR_SERVER=http://localhost:1022
NEXT_PUBLIC_SYSTEM_SERVER=http://localhost:1022
UPSTREAM_ORIGIN=http://localhost:1022
EOF
    print_ok "已写入 ${env_file}（指向 localhost:1022）"
}

# =============================================================
#  STEP 4 — 启动后端
# =============================================================
start_backend() {
    print_step 4 $TOTAL_STEPS "启动 Go 后端 (端口 $BACKEND_PORT)"

    if [ ! -f "$BACKEND_DIR/go.mod" ]; then
        print_error "后端目录不存在或缺少 go.mod: $BACKEND_DIR"
        return 1
    fi

    echo "  → cd bid-engine-backend && LOCAL_DEV=${LOCAL_DEV:-true} go run cmd/server/main.go"

    cd "$BACKEND_DIR"
    LOCAL_DEV="${LOCAL_DEV:-true}" GOFLAGS=-mod=mod go run cmd/server/main.go &
    BACKEND_PID=$!
    cd "$PROJECT_DIR"

    local backend_ready=false
    for i in $(seq 1 60); do
        if (echo > /dev/tcp/127.0.0.1/"$BACKEND_PORT") 2>/dev/null; then
            print_ok "Go 后端已就绪 (PID=$BACKEND_PID, http://localhost:$BACKEND_PORT)"
            backend_ready=true
            break
        fi
        if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
            print_error "后端进程已退出，请检查上方日志"
            return 1
        fi
        sleep 1
    done
    if [ "$backend_ready" = false ]; then
        print_warn "后端启动较慢（超过 60s），请稍后检查 (PID=$BACKEND_PID)"
    fi
}

# =============================================================
#  STEP 5 — 启动前端
# =============================================================
start_frontend() {
    print_step 5 $TOTAL_STEPS "启动 Next.js 前端 (端口 $FRONTEND_PORT)"

    if [ ! -d "$FRONTEND_DIR/node_modules" ]; then
        echo "  → 未检测到 node_modules，正在安装前端依赖（仅此一步，不涉及容器）..."
        cd "$FRONTEND_DIR"
        npm install --legacy-peer-deps
        cd "$PROJECT_DIR"
        print_ok "前端依赖安装完成"
    fi

    echo "  → cd bid-engine-frontend && PORT=$FRONTEND_PORT npm run dev"

    cd "$FRONTEND_DIR"
    PORT=$FRONTEND_PORT npm run dev &
    FRONTEND_PID=$!
    cd "$PROJECT_DIR"

    local frontend_ready=false
    for i in $(seq 1 60); do
        if (echo > /dev/tcp/127.0.0.1/"$FRONTEND_PORT") 2>/dev/null; then
            print_ok "Next.js 前端已就绪 (PID=$FRONTEND_PID, http://localhost:$FRONTEND_PORT)"
            frontend_ready=true
            break
        fi
        if ! kill -0 "$FRONTEND_PID" 2>/dev/null; then
            print_error "前端进程已退出，请检查上方日志"
            return 1
        fi
        sleep 1
    done
    if [ "$frontend_ready" = false ]; then
        print_warn "前端启动较慢（超过 60s），请稍后检查 (PID=$FRONTEND_PID)"
    fi
}

# =============================================================
#  STEP 6 — 打开浏览器
# =============================================================
open_browser() {
    print_step 6 $TOTAL_STEPS "打开浏览器"

    local url="http://localhost:$FRONTEND_PORT/signin"
    echo "  → $url"

    case "$OS" in
        macos)
            open -a "Google Chrome" "$url" 2>/dev/null || \
            open "$url" 2>/dev/null || \
            print_warn "无法自动打开浏览器"
            ;;
        windows)
            start chrome "$url" 2>/dev/null || \
            start "$url" 2>/dev/null || \
            print_warn "无法自动打开浏览器"
            ;;
        wsl)
            explorer.exe "$url" 2>/dev/null || \
            powershell.exe Start-Process "$url" 2>/dev/null || \
            print_warn "无法自动打开浏览器"
            ;;
        *)
            print_warn "不支持自动打开浏览器，请手动访问 $url"
            ;;
    esac
    print_ok "浏览器已打开"
}

# =============================================================
#  清理函数（Ctrl+C 时调用）
# =============================================================
cleanup() {
    echo ""
    echo -e "${YELLOW}${BOLD}正在停止服务...${NC}"

    if [ -n "$FRONTEND_PID" ]; then
        kill "$FRONTEND_PID" 2>/dev/null && echo -e "  ${GREEN}✓${NC} 已停止前端 (PID=$FRONTEND_PID)" || true
    fi

    if [ -n "$BACKEND_PID" ]; then
        kill "$BACKEND_PID" 2>/dev/null && echo -e "  ${GREEN}✓${NC} 已停止后端 (PID=$BACKEND_PID)" || true
    fi

    echo -e "${GREEN}${BOLD}本地服务已停止。${NC}"
    echo -e "提示: Redis / MinIO / Docling / 招标情报采集服务等 Docker 容器未受影响（本脚本不管理容器）"
    exit 0
}

# =============================================================
#  汇总输出
# =============================================================
print_summary() {
    echo ""
    echo -e "${CYAN}${BOLD}===============================================${NC}"
    echo -e "${CYAN}${BOLD}    全部启动完成！${NC}"
    echo -e "${CYAN}${BOLD}===============================================${NC}"
    echo ""
    echo -e "  ${BOLD}服务             地址${NC}"
    echo -e "  ─────────────────────────────────────"
    echo -e "  前端               ${CYAN}http://localhost:${FRONTEND_PORT}/signin${NC}"
    echo -e "  后端               ${CYAN}http://localhost:${BACKEND_PORT}${NC}"
    echo ""
    echo -e "  测试账号: ${BOLD}10000000000 / admin000000!${NC}"
    echo ""
    echo -e "  ${YELLOW}按 Ctrl+C 停止前端和后端服务${NC}"
    echo -e "  Redis / MinIO / Docling / 招标情报采集服务 / Doc-Converter 容器由你自行维护（本脚本不管理），"
    echo -e "  未运行时请手动执行: ${CYAN}docker compose -f docker-compose.yml up -d${NC}"
    echo ""
}

# =============================================================
#  主流程
# =============================================================
main() {
    trap cleanup EXIT INT TERM

    print_banner

    preflight_check       || { print_error "环境预检未通过"; exit 1; }
    free_ports
    setup_frontend_env
    start_backend         || { print_error "后端启动失败"; exit 1; }
    start_frontend        || { print_error "前端启动失败"; exit 1; }
    open_browser
    print_summary

    wait
}

main
