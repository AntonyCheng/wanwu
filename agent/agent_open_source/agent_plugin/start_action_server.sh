#!/bin/bash
#source activate action
# 基础配置
APP_NAME="http_action_server"
NUM_WORKERS=2
PORT=1992
HOST="0.0.0.0"
LOG_DIRECTORY="./logs/"
LOG_FILE="${LOG_DIRECTORY}${APP_NAME}_console.log"

# 检查日志目录是否存在，如果不存在则创建
if [ ! -d "$LOG_DIRECTORY" ]; then
  echo "日志目录不存在，正在创建..."
  mkdir -p "$LOG_DIRECTORY"
  echo "日志目录已创建：$LOG_DIRECTORY"
fi

# 根据端口号查找进程号并尝试杀死相关进程
process_ids=$(lsof -ti:$PORT)
if [ -n "$process_ids" ]; then
  echo "正在杀掉端口为 ${PORT} 的进程..."
  kill $process_ids || kill -9 $process_ids
  sleep 8  # 等待一秒以确保端口被彻底释放
  echo "已杀掉端口为 ${PORT} 的进程。"
else
  echo "未找到端口为 ${PORT} 的相关进程。"
fi

# 启动程序
echo "启动 ${APP_NAME} 程序..."
PYTHONUNBUFFERED=1 gunicorn --timeout 300   --workers $NUM_WORKERS --bind $HOST:$PORT $APP_NAME:app >$LOG_FILE 2>&1 &

# 等待程序启动一段时间
sleep 3  # 可以根据程序启动所需时间调整等待时间

# 检查端口是否有进程在监听
if lsof -i:$PORT >/dev/null; then
  echo "程序已成功启动，并输出重定向到 ${LOG_FILE}。"
else
  echo "程序启动失败，端口 ${PORT} 上未找到运行的进程，请检查 ${LOG_FILE} 了解详情。"
fi
