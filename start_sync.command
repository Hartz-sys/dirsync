#!/bin/bash
# 双击运行 dirsync 交互同步（macOS）
# 用法：把本文件放到桌面，双击即可；或在终端执行 ./start_sync.command

set -euo pipefail

DIRSYNC_HOME="/Users/houzhen/Documents/Cursor_workspace/tools/dirsync"
BIN="$DIRSYNC_HOME/dirsync"

cd "$DIRSYNC_HOME" || {
  echo "错误: 无法进入目录: $DIRSYNC_HOME"
  read -r -p "按回车键关闭..."
  exit 1
}

if [[ ! -x "$BIN" ]]; then
  echo "未找到可执行文件，正在编译..."
  if ! command -v go >/dev/null 2>&1; then
    echo "错误: 未安装 Go，且不存在已编译的 dirsync"
    read -r -p "按回车键关闭..."
    exit 1
  fi
  go build -o dirsync .
fi

clear
echo "========================================"
echo "  dirsync - Mac → Linux 目录同步"
echo "========================================"
echo

./dirsync
status=$?

echo
if [[ $status -ne 0 ]]; then
  echo "执行结束，退出码: $status"
fi
read -r -p "按回车键关闭窗口..."
exit "$status"
