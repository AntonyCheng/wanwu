#!/bin/bash

# 基础配置
APP_NAME="es_rag_server"
NUM_WORKERS=5
PORT=20041
HOST="0.0.0.0"
LOG_FILE="${APP_NAME}_console.log"

## 根据端口号查找进程号并尝试杀死相关进程
#process_ids=$(lsof -ti:$PORT)
#if [ -n "$process_ids" ]; then
#  echo "正在杀掉端口为 ${PORT} 的进程..."
#  kill $process_ids || kill -9 $process_ids
#  sleep 5  # 等待一秒以确保端口被彻底释放
#  echo "已杀掉端口为 ${PORT} 的进程。"
#else
#  echo "未找到端口为 ${PORT} 的相关进程。"
#fi

ps -ef | grep $PORT | grep -v grep | awk '{print $2}' | xargs kill -9
sleep 2

# 启动程序
echo "启动 ${APP_NAME} 程序...到${PORT}端口"
PYTHONUNBUFFERED=1 gunicorn --workers $NUM_WORKERS --bind $HOST:$PORT --timeout 600 $APP_NAME:app >$LOG_FILE 2>&1 &

if [ $? -eq 0 ]; then
  echo "程序已启动，并输出重定向到 ${LOG_FILE}。"
else
  echo "程序启动失败，请检查 ${LOG_FILE} 了解详情。"
fi
