# 定义日志文件名称
BASE_LOG_FILE="url_batch_parse_logs_"


#kill现有进程
ps -ef | grep '[u]rl_batch_parse.py' | grep -v grep | awk '{print $2}' | xargs kill -9

# 进入url_parser目录
cd url_parser

#循环5次，启动
for ADDID in $(seq -f "%03g" 1 4)  
do  
    LOG_FILE=$BASE_LOG_FILE$ADDID nohup /root/miniconda3/envs/rag-new/bin/python -u url_batch_parse.py &
    echo "应用启动成功，日志文件为./logs/$BASE_LOG_FILE$ADDID.log。"
done
