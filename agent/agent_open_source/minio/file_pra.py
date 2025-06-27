from flask import Flask, request, jsonify
from flask import Response, stream_with_context
import asyncio
import configparser
import os
from openai import OpenAI
from datetime import datetime
import json
import requests
import concurrent.futures
import logging


#from utils.build_prompt import build_docqa_prompt_from_search_list


config = configparser.ConfigParser()
config.read('/agent/agent_open_source/config.ini',encoding='utf-8')

SENTENCE_SIZE = 500
OVERLAP_SIZE = 0.0


app = Flask(__name__)




def parse_doc(file_url, sentence_size, overlap_size):
    """
    解析单个文档
    
    参数:
    file_url (str): 文件URL
    sentence_size (int): 句子大小
    overlap_size (float): 重叠比例
    user_token (str, optional): 用户token
    
    返回:
    list: 解析后的文档片段列表
    """
    
    url = config["MODELS"]["default_doc_parser_url"]
    payload = json.dumps({
        "url": file_url,
        "sentence_size": sentence_size,
        "overlap_size": overlap_size,
        "separators":[
            "\n\n", "\n", " ", ",",
            "\u200b",  # 零宽空格
            "\uff0c",  # 全角逗号
            "\u3001",  # 顿号
            "\uff0e",  # 全角句号
            "\u3002",  # 句号
            ".", "",
        ]
    })

    headers = {
        "Content_Type": "application/json;charset=utf-8"
    }

    try:
        response = requests.post(url, headers=headers, data=payload, verify=False)
        result_dict = json.loads(response.text)
        #result_dict.pop("docs")
        #print("req_chat_doc status :",{result_dict},flush=True)
        #response.raise_for_status()
        #docs = response.json().get("docs", [])
        docs = response.json().get("docs", [])
        print('docs',docs)
        return docs
    except Exception as e:
        print("parse_doc error:", {str(e)})
        return []



        
@app.route('/doc_pra', methods=['POST'])
def req_chat_doc():
    data = request.get_json()
    print("接收到请求数据：",data,flush=True)

    logging.info(f"接收到请求数据：{data}")
    file_url = data.get("upload_file_url")
    sentence_size = SENTENCE_SIZE
    overlap_size = OVERLAP_SIZE
    # 统一处理为列表
    file_urls = [file_url] if isinstance(file_url, str) else file_url
    all_docs = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
        # 创建解析任务
        future_to_url = {
            executor.submit(parse_doc, url, sentence_size, overlap_size): url 
            for url in file_urls
        }
        # 收集解析结果
        for future in concurrent.futures.as_completed(future_to_url):
            url = future_to_url[future]
            try:
                docs = future.result()
                all_docs.extend(docs)
            except Exception as e:
                print("解析文档失败",{str(e)})
    
    if not all_docs:
        return jsonify([])
    
    # 构建文档列表
    doc_list = [
        {
            "snippet": doc.get("text"),
            "file_name": doc.get("metadata", {}).get("file_name"),
        } for doc in all_docs
    ]
    return all_docs




if __name__ == '__main__':
    app.run(host='0.0.0.0', port=15003)
