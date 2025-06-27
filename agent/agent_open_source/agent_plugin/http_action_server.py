import json
import re
import logging  
import yaml
import urllib3
import os
import requests
import configparser
from utils.model_tools import req_unicom_llm_chat  #导入第三方工具模型
from utils.actions.agents import Assistant,ReActChat
from utils.actions.tools.base import BaseTool, register_tool
from utils.actions.llm import get_chat_model
from utils.actions.tools.openapi_plugin import openapi_schema_convert,add_openapi_plugin_to_additional_tool
from utils.generator import *
from flask import  stream_with_context, Response, request
from flask import Flask
from log_config import setup_logging
from flask_cors import CORS
from utils.model_tools import *


###设置日志
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
setup_logging("http_action_server")
logger = logging.getLogger("http_action_server")
config = configparser.ConfigParser()
config.read('config.ini')

###配置鉴权信息
APP_ID = config["ACTION"]['APP_ID']
API_KEY = config["ACTION"]['API_KEY']
SECRET_KEY = config["ACTION"]['SECRET_KEY']

MODEL_NAME_CONFIG = config["MODELS"]["default_llm"]
MODEL_NAME = os.getenv('CUAI_DEFAULT_LLM_MODEL_ID', MODEL_NAME_CONFIG)

MODEL_URL_CONFIG = config["MODELS"]["model_url"]
MODEL_URL = os.getenv('CUAI_DEFAULT_LLM_MODEL_URL', MODEL_URL_CONFIG)


app = Flask(__name__)
CORS(app, supports_credentials=True)

import re
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


def add_link_tags(text):
    ####判断原始输出是否已是markdown格式
    markdown_pattern = r'(?:!\[(.*?)\]|\[(.*?)\])\((.+?)\)'
    # 定义图片文件扩展名和非图片文件扩展名的正则表达式模式  (不含签名格式)
    image_extensions = r'\.(?:jpeg|jpg|png|webp)'
    non_image_extensions = r'\.(?:wav|mp3|mp4|m4a|txt|pdf|docx|xlsx|html)'
    # 定义链接的正则表达式模式，匹配以指定扩展名结尾的链接
    link_pattern = r'https?://[^\s]+(?:' + image_extensions + '|' + non_image_extensions + ')'
    # link_pattern = r'(?:!\[(.*?)\]|\[(.*?)\])\((.+?)\)' + '|' + r'https?://[^\s]+(?:' + image_extensions + '|' + non_image_extensions + ')'


    # 定义图片文件扩展名和非图片文件扩展名的正则表达式模式  (含签名格式)
    # image_extensions_sign = r'\.(?:jpeg\?|jpg\?|png\?|webp\?)'
    # non_image_extensions_sign = r'\.(?:wav\?|mp3\?|mp4\?|m4a\?|txt\?|pdf\?|docx\?|xlsx\?|html\?)'
    text = text.replace("https//", "https://").replace("http//", "http://") ##规范化大模型的输出
    # 查找所有匹配的链接
    links = re.findall(link_pattern, text)
    logger.info(f"-------add_link_tags links:{links}-------")

    for link_tuple in links:
        logger.info(f"link_tuple:{link_tuple}")

        # 提取元组中的信息
        # 对于标准 Markdown 链接：('', '显示文本', 'URL')
        # 对于普通 URL：('URL', '', '')
        display_text = link_tuple[1] if link_tuple[1] else link_tuple[0]
        url = link_tuple[2] if link_tuple[2] else link_tuple[0]

        if not url:
            continue  # 跳过无效链接

        # 判断链接类型并添加对应标签
        if re.search(image_extensions, url):
            # 如果是图片链接，添加图片标签
            tag = f'![{display_text}]({url})'
        elif re.search(non_image_extensions, url):
            # 如果是非图片链接，添加普通链接标签
            tag = f'[{display_text}]({url})'
        else:
            # 其他情况保持原样
            tag = url

        # 替换原始文本中的链接部分
        # 注意：这里使用原始匹配的文本部分进行替换，确保准确替换
        original_link_text = f'[{display_text}]({url})' if display_text else url
        text = text.replace(original_link_text, tag)

    logger.info(f"-------add_link_tags text:{text}-------")
    return text



def plugin_config(API_KEY,query,plugin_list,function_calls_list,action_type,history,model,model_url):
    ###配置模型服务
    llm = get_chat_model({
        # Use the model service provided by DashScope:
        'model': 'unicom-72b-chat-ali-v2',
        'model_server': 'http://1654416620085582.cn-wulanchabu.pai-eas.aliyuncs.com/api/predict/llm_70b_2/v1/chat/completions',
        'api_key': API_KEY,

        # Use the OpenAI-compatible model service provided by DashScope:
        # 'model': 'qwen1.5-14b-chat',
        # 'model_server': 'https://dashscope.aliyuncs.com/compatible-mode/v1',
        # 'api_key': os.getenv('DASHSCOPE_API_KEY'),

        # Use the model service provided by Together.AI:
        # 'model': 'Qwen/Qwen1.5-14B-Chat',
        # 'model_server': 'https://api.together.xyz',  # api_base
        # 'api_key': os.getenv('TOGETHER_API_KEY'),

        # Use your own model service compatible with OpenAI API:
        # 'model': 'Qwen/Qwen1.5-72B-Chat',
        # 'model_server': 'http://localhost:8000/v1',  # api_base
        # 'api_key': 'EMPTY',
    })
    
    try:
        messages_input = []
        logger.info(f"---history为：{history}---")
        if history:
            has_rewrite_query = any("rewrite_query" in item.keys() for item in history)
            if has_rewrite_query:
                for item in history:         
                    messages_input.append({"role": "user", "content":item["rewrite_query"]})
                    messages_input.append({"role": "assistant", "content":item["response"]})    
            else:
                history = [item for item in history if item['role'] != "system"]
                for i in range(0,len(history),2):
                    messages_input.append({"role": "user", "content":history[i]['content']})
                    messages_input.append({"role": "assistant", "content":history[i+1]["content"]})  

        messages_input.append({'role': 'user', 'content': query})
        name = "unicomllm"
        logger.info("\n---------识别action插件并进行配置-------------")
        function_list = []
        # logger.info(f"原始的plugin_list：{plugin_list}")
        for i in range(0,len(plugin_list)):
            api_schema = plugin_list[i]['api_schema']  
            if 'api_auth' in plugin_list[i]:
                api_auth = plugin_list[i]['api_auth']
            else:
                ####非鉴权action适配
                api_auth = {
                'type': None,  
                'in': 'header',  
                'name': 'Authorization', 
                'value': None
            }  
            api_cfg = openapi_schema_convert(api_schema,api_auth)
            # logger.info(f"api_cfg：{api_cfg}")
            if api_cfg:
                plugin_cfg = api_cfg
                # 注册function
                fn_list = add_openapi_plugin_to_additional_tool(plugin_cfg, [])
                function_list.extend(fn_list)
        
        logger.info(f"---完成action插件并进行配置，function_list为：{function_list}---")
        if action_type == "modelscope_agent":
            system_message = "你是一个API接口查询助手,你需要选择最优的API接口并严格按照API的入参要求来识别参数，在识别参数时，严禁进行任何形式的自由发挥，必须严格按照用户提供的信息来确定参数"
        elif action_type == "function_call":
            system_message = "作为function查询助手，你的任务是基于用户问题精准地识别出相应的函数及参数。在识别参数时，严禁进行任何形式的自由发挥，必须严格按照用户提供的信息来确定参数"
        else:
            system_message = "你是一个任务规划助手，你需要基于已有的工具对用户问题进行拆分成多个任务task，并对每个任务进行分步推理,请注意，后一步的推理务必用到前一步的结果，最后，请逐步输出结果。在识别参数时，严禁进行任何形式的自由发挥，必须严格按照用户提供的信息来确定参数"
        logger.info("-------------启动llm_actions函数------------")

        llm_actions = ReActChat(function_list = function_list,llm=llm,system_message = system_message,name = name,function_calls_list=function_calls_list,action_type = action_type,model_name = model,model_url = model_url)

        logger.info(f"messages_input：{messages_input}")
        action_response = llm_actions.run(messages_input)
        logger.info(f"action_response 的类型为：{type({action_response})}")

        total_result = ""
        id = 0
        prompt_tokens = len("".join([str(item) for item in messages_input]))
        completion_tokens = 0
        total_tokens = 0

        for line in action_response:
            if isinstance(line[0],str):
                line = line[0] 
            else:
                line = line[0]['content']
            total_result += line

            try:
                line = json.loads(line)
            except Exception as e:
                line = line

            if ("<tool>" not in total_result or "</tool>" not in total_result or "</tool>" in line) and (not isinstance(line,dict)):
                completion_tokens += len(line)
                prompt_tokens = prompt_tokens
                total_tokens = completion_tokens + prompt_tokens

                line = line + "\n\n" if "</tool>" in line else line  ###适配链接的markdown输出
                result = {"code": 0, "data": {"choices": [{"finish_reason": "", "index": 0, "message": {"content": line, "role": "assistant"}}], "model": model, "object": "chat.completion", "usage": {"completion_tokens": completion_tokens, "prompt_tokens": prompt_tokens, "total_tokens": total_tokens}}, "msg": "ok"}
                jsonarr = json.dumps(result, ensure_ascii=False)
                str_out = f'id:{id}\nevent:result\ndata:{jsonarr}\n\n'
                yield str_out 
                
            ####特殊情况处理：API原始为流式结果，直接输出结果    
            elif ("<tool>"  in total_result and "</tool>"  in total_result) and (not isinstance(line,dict)):
                completion_tokens += len(line)
                prompt_tokens = prompt_tokens
                total_tokens = completion_tokens + prompt_tokens
                result = {"code": 0, "data": {"choices": [{"finish_reason": "", "index": 0, "message": {"content": line, "role": "assistant"}}], "model": model, "object": "chat.completion", "usage": {"completion_tokens": completion_tokens, "prompt_tokens": prompt_tokens, "total_tokens": total_tokens}}, "msg": "ok"}
                jsonarr = json.dumps(result, ensure_ascii=False)
                str_out = f'id:{id}\nevent:result\ndata:{jsonarr}\n\n'
                yield str_out

            ####特殊情况处理：API原始为流式结果，直接输出结果
            elif ("<tool>" in total_result and "</tool>" in total_result) and ((not isinstance(line,dict)) or ((isinstance(line,dict)) and "action_code" not in line.keys())):
                completion_tokens += len(line)
                prompt_tokens = prompt_tokens
                total_tokens = completion_tokens + prompt_tokens
                result = {"code": 0, "data": {
                    "choices": [{"finish_reason": "", "index": 0, "message": {"content": line, "role": "assistant"}}],
                    "model": model, "object": "chat.completion",
                    "usage": {"completion_tokens": completion_tokens, "prompt_tokens": prompt_tokens,
                              "total_tokens": total_tokens}}, "msg": "ok"}
                jsonarr = json.dumps(result, ensure_ascii=False)
                str_out = f'id:{id}\nevent:result\ndata:{jsonarr}\n\n'
                yield str_out

            else:
                logger.info(f"total_result:{total_result}")
                if line['action_code'] in [2]  or  ("<tool>" in total_result and line['action_code'] in [0]):
                    action_output = line['action_output']
                elif "<tool>" in total_result and line['action_code'] in [1]:
                    total_result = total_result + f"请基于以上问题直接问答用户问题:{query}"
                    messages_input.append({"role": "assistant", "content": total_result})
                    action_output = req_unicom_llm_chat_plus(messages=messages_input, model_name=model, model_url=model_url)
                    action_output = re.sub('<tool>.*?</tool>', '', action_output, flags=re.DOTALL)
                    action_output = "</tool>\n\n" + action_output if "</tool>" not in total_result else action_output
                else:
                    action_output = ""
                logger.info(f"action_output:{action_output}")
                ###正则匹配图片、文件链接
                if action_output:
                    action_output = add_link_tags(action_output)
                    logger.info(f"add_link_tags action_output:{action_output}")
                    for t in range(0,len(action_output),5):
                        split_action_output =action_output[t:t+5]
                        completion_tokens += len(split_action_output)
                        prompt_tokens = prompt_tokens
                        total_tokens = completion_tokens + prompt_tokens
                        result = {"code": 0, "data": {"choices": [{"finish_reason": "", "index": 0, "message": {"content": split_action_output, "role": "assistant"}}], "model": model, "object": "chat.completion", "usage": {"completion_tokens": completion_tokens, "prompt_tokens": prompt_tokens, "total_tokens": total_tokens}}, "msg": "ok"}
                        jsonarr = json.dumps(result, ensure_ascii=False)
                        str_out = f'id:{id}\nevent:result\ndata:{jsonarr}\n\n'
                        yield str_out
                        id += 1

                    ####增加流式输出结束点
                    result = {"code": 0, "data": {"choices": [{"finish_reason": "stop", "index": 0,"message": {"content": "","role": "assistant"}}], "model": model,"object": "chat.completion","usage": {"completion_tokens": completion_tokens,"prompt_tokens": prompt_tokens, "total_tokens": total_tokens}},"msg": "ok"}
                    jsonarr = json.dumps(result, ensure_ascii=False)
                    str_out = f'id:{id}\nevent:result\ndata:{jsonarr}\n\n'
                    yield str_out
            id += 1
                        
    except Exception as e:
        error_msg = str(e)
        arr = {}
        arr['action_code'] = 1
        arr['message'] = error_msg
        arr['result'] = ''
        jsonarr = json.dumps(arr, ensure_ascii=False)
        logger.info(f"jsonarr: {jsonarr}")
        yield jsonarr
        
@app.route("/agent/action",methods=['POST'])
def action_infer():
    data = request.get_json()
    logger.info('request_params: '+ json.dumps(data, ensure_ascii=False))
    query = data.get("input")
    model_name = data.get("model_name",MODEL_NAME)
    plugin_list = data.get("plugin_list",[])
    function_calls_list = data.get("function_calls_list",[])
    action_type =  data.get("action_type")  
    history =  data.get("history")
    model = data.get("model_name",MODEL_NAME)
    model_url = data.get("model_url","")
    use_search = data.get("use_search",False)

    logger.info(f"query is {query}")
    logger.info(f"model is {model}")
    logger.info(f"model_url is {model_url}")
    logger.info(f"plugin_list 的数量为 {len(plugin_list)}")
    logger.info(f"function_calls_list 的数量为 {len(function_calls_list)}")
    action_result = plugin_config(API_KEY,query,plugin_list,function_calls_list,action_type,history,model,model_url)
    logger.info(f"action_result 的类型为：{type({action_result})}")
    
    try:
        if next(action_result):
            return action_result
        
    except Exception as e:
        pass
    
        
    
        
if __name__ == '__main__':
    logger.info("http_action_server start")
    app.run(host='0.0.0.0', port=1992, threaded=False,debug=False)
   
    




