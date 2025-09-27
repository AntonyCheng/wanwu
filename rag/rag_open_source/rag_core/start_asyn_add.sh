# 定义日志文件名称
BASE_LOG_FILE="asyn_add_"

#kill现有进程
ps -ef | grep '[a]syn_add_file.py' | grep -v grep | awk '{print $2}' | xargs kill -9
# 发送重启信号
eval "$(conda shell.bash hook)"
conda activate rag-new
python -u asyn_doc_status_init.py > logs/init_asyn.out 2>&1
sleep 1

#循环5次，启动asyn_add
for ADDID in $(seq -f "%03g" 1 2)
do  
    LOG_FILE=$BASE_LOG_FILE$ADDID nohup python -u asyn_add_file.py >>logs/start_asyn_add.out 2>&1 &
    echo "应用启动成功，日志文件为./logs/$BASE_LOG_FILE$ADDID.log。"
done
