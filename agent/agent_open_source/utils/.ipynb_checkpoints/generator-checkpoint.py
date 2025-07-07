import json
import requests
import logging
import datetime
import re
import copy
import configparser
from django.http import StreamingHttpResponse, JsonResponse,HttpResponse
from llm_agent_server_app.utils.qa_types import qa_types

from llm_agent_server_app.utils.output_parser import extract_json
from llm_agent_server_app.utils.redis_db import RedisClient

from langchain_core.messages import (
    AIMessage,
    AIMessageChunk,
)

QUERY_SHORT=10

logger = logging.getLogger(__name__)

# 创建 RedisClient 实例
#redis_client = RedisClient()

config = configparser.ConfigParser()
config.read('config.ini',encoding='utf-8')

Default_HISTORY_NUMS = int(config["AGENTS"]["Default_HISTORY_NUMS"])

# def create_error_response(msg):
#     """构造错误响应的函数，并设置状态码"""
#     return Response(json.dumps({'code': 2, 'msg': msg, 'response': ''},ensure_ascii=False), status=400, mimetype='application/json')
def create_error_response(msg):
    """构造错误响应的函数，并设置状态码"""
    response_data = {
        'code': 2,
        'msg': msg,
        'response': ''
    }
    return JsonResponse(response_data, status=400, json_dumps_params={'ensure_ascii': False})


def clean_response_content(content):
    """
    清除响应内容中的引文标记和思考过程标签
    Args:
        content: 原始响应内容
    Returns:
        清除标记后的内容
    """
    return re.sub(r"【\d+\^】|<think>.*?</think>", "", content, flags=re.DOTALL)
    
def generate_stream_response(query, rewrite_query, prompt, response, qa_type, history,ignore_in_history = False, need_search_list=True,search_list=[],gen_file_url_list=[],upload_file_url='',non_llm_response=None,func_name = '',func_params = '',thought_inference = '',request_begin_time = None,request_id="",session_id="", agent_id="",resourceId = ""):
    """
    模拟流式返回API响应的生成器函数，根据提供的响应内容分步骤生成流式响应。
    :param question: 提问内容。
    :param prompt: 提示信息。
    :param full_response: 完整的响应文本。
    :param history: 历史交互记录列表。
    :param need_search_list: 是否需要搜索列表。
    :param search_list: 可选的搜索列表。
    :return: 生成流式响应的单个部分。
    """
    # now = datetime.datetime.now()
       
    # cur_date = now.strftime(f"%Y-%m-%d-%H:%M:%S")
    
    logger.info(f"{request_id} {query[:QUERY_SHORT]}--->stream response start")
    logger.info(f"{request_id} {query[:QUERY_SHORT]}--->need_search_list: {need_search_list}")
    # logger.info(f"{request_id} {query[:QUERY_SHORT]}--->search_list: {search_list}")  
    id = 0
    if history:
        history_tmp = history.copy()
    else:
        history_tmp = []
    
    # 定义全局变量，流式结束原因：0（生成中），1（正常结束），2（非正常结束，输出超过最大长度）
    finish_reason = 0

    # llm_sucess_flag = []
    llm_failed_flag = False

    if response:
        
        incremental_content  = ""
        complete_content = ""
        infer_content = ""
        thought = ""
        
        # buffer =""
        # pattern = re.compile(r"<sup class='citation'>(\d+)</sup>")
        
        if qa_type in [qa_types["ACTION"],qa_types["FUNC_CALL"]]  and thought_inference:
            logger.info("------start thought_inference response-----")
            
            for i in range(0, len(thought_inference), 10):

                infer_content = thought_inference[i:i + 10]
                thought += thought_inference[i:i + 10]
                finish_reason = 0

                usage = {
                    "prompt_tokens": len(infer_content),
                    "completion_tokens": len(infer_content),
                    "total_tokens": len(thought_inference)
                }

                result = {
                    "code": 0,
                    "agent_id":agent_id, 
                    "session_id":session_id, 
                    "request_id":request_id, 
                    "message": "success",
                    "response": "",
                    "gen_file_url_list": gen_file_url_list,
                    "qa_type": qa_type,
                    "func_name":func_name,
                    "func_params":func_params,
                    "thought_inference":infer_content,
                    "history":history_tmp,                
                    "finish": finish_reason,
                    "usage": usage
                }

                if need_search_list:
                    result["search_list"] = search_list
                
                jsonarr = json.dumps(result, ensure_ascii=False)
                id += 1
                str_out = f'id:{id}\nevent:result\ndata:{jsonarr}\n\n'          
                yield str_out
        

        if isinstance(response, requests.models.Response):
            # http服务方式的 response，来自语言或编码模型
            # for line in response.iter_lines(decode_unicode=True):
            # logger.info(f"{request_id} {query[:QUERY_SHORT]} ---> 原始大模型llm_response:\n{response.text}" ) 
            
            try:
                for i, line in enumerate(response.iter_lines(decode_unicode=True)):
                    
                    if i==0 and request_begin_time :
                        first_token_time = datetime.datetime.now()
                        first_token_delay = first_token_time - request_begin_time
                        logger.info(f"{request_id} {query[:QUERY_SHORT]}---> Fisrt token delay:{first_token_delay}   " )    
                    
                    
                    if isinstance(line,list) and isinstance(line[0]['content'],str):
                        line = line[0]['content']
                    if isinstance(line,list) and isinstance(line[0]['content'][0]['text'],str):
                        line = line[0]['content'][0]['text']
                    if line.startswith("data:") and not line.startswith('data:""'):
                        # logger.info(f"line_test:{line}")
                        line = line[5:]
                        datajson = json.loads(line)
                        # logger.info(f"{datajson}")
                        # print(datajson)
    
                        incremental_content = datajson["data"]["choices"][0]["message"]["content"]
                        
                        complete_content += datajson["data"]["choices"][0]["message"]["content"]
                        
                        if qa_type in [qa_types["ACTION"],qa_types["FUNC_CALL"]] :
                            
                            infer_content = datajson["data"]["choices"][0]["message"]["content"]
                            thought += datajson["data"]["choices"][0]["message"]["content"]
                            
    
                        if not datajson['data']['choices'][0]['finish_reason']:
                            finish_reason = 0
    
                            # subjson["needHistory"] = False
                            # subjson["response"] = incremental_content
                            # history_tmp.append(subjson)
                            result = {
                                "code": 0,
                                "agent_id":agent_id,  
                                "session_id":session_id, 
                                "request_id":request_id, 
                                "message": "success",
                                "response": incremental_content,
                                "gen_file_url_list": gen_file_url_list,
                                "qa_type": qa_type,
                                "func_name":func_name,
                                "func_params":func_params,
                                "thought_inference":infer_content,
                                "history": [],
                                "finish": finish_reason,
                                "usage": datajson['data']['usage'],
                                "model": datajson['data']['model']
                            }
                            if i==2 and need_search_list:
                                result["search_list"] = search_list
                                # logger.info(f"{request_id} {query[:QUERY_SHORT]}--->model_name: {datajson.get('data',{}).get('model')}")
                                
    
                            jsonarr = json.dumps(result, ensure_ascii=False)
                            id += 1
                            str_out = f'id:{id}\nevent:result\ndata:{jsonarr}\n\n'
    
                            yield str_out
    
                        else:
                            if datajson['data']['choices'][0]['finish_reason']=="stop":  
                                # 正常结束
                                finish_reason = 1
                            if datajson['data']['choices'][0]['finish_reason']=="length":
                                # 长度被截断
                                finish_reason = 2

                            if datajson['data']['choices'][0]['finish_reason']=="sensitive_cancel":
                                # 敏感词
                                finish_reason = 4


                            if not gen_file_url_list:
                                gen_file_url_list = datajson.get("gen_file_url_list",[])
                                
                            if not ignore_in_history: 
                             
                                subjson = {}
                                subjson["agent_id"]=agent_id
                                subjson["session_id"]=session_id
                                subjson["request_id"]=request_id
                                subjson["query"] = query
                                subjson["rewrite_query"] = rewrite_query
                                subjson["upload_file_url"] = upload_file_url
                                # subjson["prompt"] = prompt
                                # print(datajson['data']['choices'][0]['finish_reason'])
                            # 如果是代码解释器类型 且 代码解释器反馈结果（datajson）中的gen_file_url_list不为空时，新加一段关于生成文件的markdown话术
                                # print(gen_file_url_list)
                                if qa_type == 4 and len(gen_file_url_list)>0:    
                                    gen_file_url = gen_file_url_list[0]["output_file_url"]
                                    if gen_file_url:
                                        if gen_file_url.endswith(('.jpeg', '.jpg', '.png','.webp')):   
                                            file_markdown_format = f" ![已处理好的文件]({gen_file_url})"
                                        else:
                                            file_markdown_format = f" [已处理好的文件]({gen_file_url})"
    
                                        complete_content = complete_content + "\n 已处理好的文件如下：\n" +  file_markdown_format
                                        incremental_content = incremental_content  + "\n 已处理好的文件如下：\n" +  file_markdown_format
                            
                                # 在加入对话历史前将引文脚本和思考过程标签清空                                                              
                                subjson["response"] = clean_response_content(complete_content)
                                subjson["gen_file_url_list"] = gen_file_url_list
                                subjson["qa_type"] = qa_type
                                subjson["func_name"] = func_name
                                subjson["func_params"] = func_params
                                subjson["thought_inference"] = thought
                                history_tmp.append(subjson)

                                if session_id:

                                    memory_prompt={"role": "user", "content":prompt}
                                    memory_prompt_response = {"role": "assistant", "content":subjson["response"]}

                                    ##redis_client.set_value(session_id,memory_prompt)                                    
                                    ##redis_client.set_value(session_id,memory_prompt_response)

                                    # memory_rewrite_query = {"role": "user", "content":rewrite_query}
                                    # memory_rewrite_query_response= {"role": "assistant", "content":subjson["response"]}

                                    #prompt不透传给用户，只存云端
                                    memory_rewrite_query = copy.deepcopy(subjson)                                    
                                    memory_rewrite_query["prompt"] = prompt

                                    ##redis_client.set_value(f"{session_id}_requery",memory_rewrite_query)

                                                             
                            result = {
                                "code": 0,
                                "agent_id":agent_id, 
                                "session_id":session_id, 
                                "request_id":request_id,                            
                                "message": "success",
                                "response": incremental_content,
                                "gen_file_url_list": gen_file_url_list,
                                "qa_type": qa_type,
                                "func_name":func_name,
                                "func_params":func_params,
                                "thought_inference":infer_content,
                                "finish": finish_reason,
                                "usage": datajson['data']['usage'],
                                "model": datajson['data']['model']
                            }
                            
                            if need_search_list:
                                result["search_list"] = search_list

                            logger.info(f'{request_id} {query[:QUERY_SHORT]}--->generate_stream answer-->:{complete_content}')
                            logger.info(f'{request_id} {query[:QUERY_SHORT]}--->generate_stream_response-->:{json.dumps(result, ensure_ascii=False)}')
                            
                            result["history"] = history_tmp
    
                            
    
                            jsonarr = json.dumps(result, ensure_ascii=False)
                            id += 1
                            str_out = f'id:{id}\nevent:result\ndata:{jsonarr}\n\n'
                            
    
                            yield str_out

            except Exception as e:
                msg_error = "当前服务端系统故障，我们将尽快解决，请您稍后重试~"
                error_dict = extract_json(str(e))
                msg_error = error_dict.get("msg", msg_error)
                # 记录日志
                logger.info(f"{request_id} {query[:QUERY_SHORT]}--->{msg_error}--->具体报错：{str(e)}")
                # 生成返回结果
                result = {
                    "code": 0,
                    "agent_id": agent_id,
                    "session_id": session_id,
                    "request_id": request_id,
                    "message": msg_error,
                    "response": "",
                    "gen_file_url_list": [],
                    "qa_type": qa_type,
                    "func_name": "",
                    "func_params": "",
                    "thought_inference": "",
                    "history": [],
                    "finish": 3,
                    "usage": {}
                }

                if need_search_list:
                    result["search_list"] = search_list

                jsonarr = json.dumps(result, ensure_ascii=False)
                id += 1
                str_out = f'id:{id}\nevent:result\ndata:{jsonarr}\n\n'

                logger.info(f'{request_id} {query[:QUERY_SHORT]}--->generate_stream_response:{jsonarr}')

                yield str_out
                
             
            except GeneratorExit as e:
                logger.info(f"{request_id} {query[:QUERY_SHORT]}--->GeneratorExit:{str(e)}") 
                logger.info(f"{request_id} {query[:QUERY_SHORT]}--->The request is terminated:{complete_content}") 
                result = {
                    "code": 0,
                    "agent_id":agent_id, 
                    "session_id":session_id, 
                    "request_id":request_id,                            
                    "message": "The request is terminated.",
                    "response": "",
                    "gen_file_url_list": [],
                    "qa_type": qa_type,
                    "func_name":"",
                    "func_params":"",
                    "thought_inference":"",
                    "history": [],
                    "finish": 3,
                    "usage": {},
                    "model":""
                }

                if need_search_list:
                    result["search_list"] = search_list

                jsonarr = json.dumps(result, ensure_ascii=False)
                id += 1
                str_out = f'id:{id}\nevent:result\ndata:{jsonarr}\n\n'
                logger.info(f'{request_id} {query[:QUERY_SHORT]}--->generate_stream_response:{jsonarr}')  
                yield str_out
                 
            finally:
                logger.info(f"{request_id} {query[:QUERY_SHORT]}--->stream response end")

        else:
            # langchain sdk返回结果
            try:
                for i,line in enumerate(response):   
                    if i==0 and request_begin_time :
                        first_token_time = datetime.datetime.now()
                        first_token_delay = first_token_time - request_begin_time
                        logger.info(f"{request_id} {query[:QUERY_SHORT]}--->Fisrt token delay:{first_token_delay}")

                    ###区分function_call的输出
                    if line.response_metadata.get("finish_reason", "") == "tool_calls":
                        incremental_content = f"<mcp>\n\n\n```mcp tools\n" + json.dumps(line.tool_calls,ensure_ascii= False) + "\n```\n\n"
                    elif line.type == "tool":
                        incremental_content = f"\n\n\n```mcp 调用结果\n" + line.content + "\n```\n\n" + "</mcp>\n\n"
                    else:
                        incremental_content = line.content            
                    complete_content += incremental_content
                    
                    model = line.response_metadata.get("model_name","") 
                    if not line.response_metadata.get("finish_reason","") or line.response_metadata.get("finish_reason", "") == "tool_calls":
                        finish_reason = 0
                        result = {
                            "code": 0,
                            "agent_id":agent_id, 
                            "session_id":session_id, 
                            "request_id":request_id,
                            "message": "success",
                            "response": incremental_content,
                            "gen_file_url_list": gen_file_url_list,
                            "qa_type": qa_type,
                            "func_name":func_name,
                            "func_params":func_params,
                            "thought_inference":"",
                            "history": [],
                            "finish": finish_reason,
                            "usage": {}, # 流式过程不必赋值
                            "model": model
                        }


                        if i==0 and need_search_list and search_list:
                            result["search_list"] = search_list
                            logger.info(f"{request_id} {query[:QUERY_SHORT]}--->len(search_list): {len(search_list)}")

                        jsonarr = json.dumps(result, ensure_ascii=False)
                        id += 1
                        str_out = f'id:{id}\nevent:result\ndata:{jsonarr}\n\n'
                        # print(str_out)

                        yield str_out
  
                    else:

                        if line.response_metadata.get("finish_reason") == "stop":  
                            # 正常结束
                            finish_reason = 1
                        if line.response_metadata.get("finish_reason") in ["length","content_filter"]:
                            # 长度被截断
                            finish_reason = 2

                        # if line.response_metadata.get("finish_reason") =="sensitive_cancel":
                        #     # 长度被截断
                        #     finish_reason = 4

                        

                        if not ignore_in_history: 
                            subjson = {}
                            subjson["agent_id"]=agent_id
                            subjson["session_id"]=session_id
                            subjson["request_id"]=request_id
                            subjson["query"] = query
                            subjson["rewrite_query"] = rewrite_query
                            subjson["upload_file_url"] = upload_file_url
                            # subjson["prompt"] = prompt
                            # print(datajson['data']['choices'][0]['finish_reason'])
                            # print(gen_file_url_list)    
                            # # 在加入对话历史前将引文脚本和思考过程标签清空                                                              
                            subjson["response"] = clean_response_content(complete_content)
                            subjson["gen_file_url_list"] = gen_file_url_list
                            subjson["qa_type"] = qa_type
                            subjson["func_name"] = func_name
                            subjson["func_params"] = func_params
                            subjson["thought_inference"] = ""
                            history_tmp.append(subjson)

                            if session_id:
                                # history_tmp = []
                                memory_prompt={"role": "user", "content":prompt}
                                memory_prompt_response = {"role": "assistant", "content":subjson["response"]}
                                
                                # memory_rewrite_query = {"role": "user", "content":rewrite_query}
                                # memory_rewrite_query_response= {"role": "assistant", "content":subjson["response"]}
                                
                                
                                #redis_client.set_value(session_id,memory_prompt)                                    
                                #redis_client.set_value(session_id,memory_prompt_response)

                                # #redis_client.set_value(f"{session_id}_requery",memory_rewrite_query)
                                #redis_client.set_value(f"{session_id}_requery",subjson)

                        input_tokens = line.usage_metadata.get("input_tokens",0)
                        output_tokens = line.usage_metadata.get("output_tokens",0)
                        total_tokens = line.usage_metadata.get("total_tokens",0)
                        usage = {'prompt_tokens': input_tokens, 'completion_tokens': output_tokens, 'total_tokens': total_tokens}
                                        
                        
                        result = {
                            "code": 0,
                            "agent_id":agent_id, 
                            "session_id":session_id, 
                            "request_id":request_id, 
                            "message": "success",
                            "response": incremental_content,
                            "gen_file_url_list": gen_file_url_list,
                            "qa_type": qa_type,
                            "func_name":func_name,
                            "func_params":func_params,
                            "thought_inference":"",
                            "finish": finish_reason,
                            "usage":  usage,
                            "model": model
                        }
                        if need_search_list:
                            result["search_list"] = search_list
                        
                        logger.info(f'{request_id} {query[:QUERY_SHORT]}--->generate_stream answer-->:{complete_content}')
                        logger.info(f'{request_id} {query[:QUERY_SHORT]}--->generate_stream_response-->:{json.dumps(result, ensure_ascii=False)}')
                        
                        
                        result["history"] =history_tmp

                        jsonarr = json.dumps(result, ensure_ascii=False)
                        id += 1
                        str_out = f'id:{id}\nevent:result\ndata:{jsonarr}\n\n'
                        

                        yield str_out

            
            except Exception as e:
                msg_error = "当前服务端系统故障，我们将尽快解决，请您稍后重试~"
                error_dict = extract_json(str(e))
                msg_error = error_dict.get("msg", msg_error)
                # 记录日志
                logger.info(f"{request_id} {query[:QUERY_SHORT]}--->{msg_error}--->具体报错：{str(e)}")

                # 生成返回结果
                result = {
                    "code": 0,
                    "agent_id": agent_id,
                    "session_id": session_id,
                    "request_id": request_id,
                    "message": msg_error,
                    "response": "",
                    "gen_file_url_list": [],
                    "qa_type": qa_type,
                    "func_name": "",
                    "func_params": "",
                    "thought_inference": "",
                    "history": [],
                    "finish": 3,
                    "usage": {}
                }

                if need_search_list:
                    result["search_list"] = search_list

                jsonarr = json.dumps(result, ensure_ascii=False)
                id += 1
                str_out = f'id:{id}\nevent:result\ndata:{jsonarr}\n\n'

                logger.info(f'{request_id} {query[:QUERY_SHORT]}--->generate_stream_response:{jsonarr}')

                yield str_out

                
            except GeneratorExit as e :
                logger.info(f"{request_id} {query[:QUERY_SHORT]}--->GeneratorExit:{str(e)}")
                logger.info(f"{request_id} {query[:QUERY_SHORT]}--->The request is terminated:{complete_content}") 
                result = {
                    "code": 0,
                    "agent_id":agent_id, 
                    "session_id":session_id, 
                    "request_id":request_id,                            
                    "message": "The request is terminated.",
                    "response": "",
                    "gen_file_url_list": [],
                    "qa_type": qa_type,
                    "func_name":"",
                    "func_params":"",
                    "thought_inference":"",
                    "history": [],
                    "finish": 3,
                    "usage": {},
                    "model":""
                }

                if need_search_list:
                    result["search_list"] = search_list

                jsonarr = json.dumps(result, ensure_ascii=False)
                id += 1
                str_out = f'id:{id}\nevent:result\ndata:{jsonarr}\n\n'
                logger.info(f'{request_id} {query[:QUERY_SHORT]}--->generate_stream_response:{jsonarr}')  
                yield str_out
                 
            finally:
                logger.info(f"{request_id} {query[:QUERY_SHORT]}--->stream response end")

    # end if response
    
    # LLM模型输出为空    
    elif non_llm_response or non_llm_response=="":
        #非语言模型的消息，模拟流式结果
        try:
            finish_reason = 1
            if qa_type in [qa_types["ACTION"],qa_types["FUNC_CALL"]] :
                thought_inference = thought_inference+"\n"+non_llm_response
            if not ignore_in_history:
                subjson = {}
                subjson["agent_id"]=agent_id
                subjson["session_id"]=session_id
                subjson["request_id"]=request_id
                subjson["query"] = query
                subjson["rewrite_query"] = rewrite_query
                subjson["upload_file_url"] = upload_file_url
                # subjson["prompt"] = prompt                
                subjson["response"] = clean_response_content(non_llm_response)
                subjson["gen_file_url_list"] = gen_file_url_list  
                subjson["qa_type"] = qa_type   
                subjson["func_name"] = func_name
                subjson["func_params"] = func_params
                subjson["thought_inference"] = thought_inference
                history_tmp.append(subjson)

                if session_id:
                    # history_tmp = []
                    memory_prompt={"role": "user", "content":prompt}
                    memory_prompt_response = {"role": "assistant", "content":subjson["response"]}
                    
                    # memory_rewrite_query = {"role": "user", "content":rewrite_query}
                    # memory_rewrite_query_response= {"role": "assistant", "content":subjson["response"]}
                    
                    
                    #redis_client.set_value(session_id,memory_prompt)                                    
                    #redis_client.set_value(session_id,memory_prompt_response)

            
            usage = {
                "prompt_tokens": 0,
                "completion_tokens": 0,
                "total_tokens": 0
            }
        
            result = {
                "code": 0,
                "agent_id":agent_id, 
                "session_id":session_id, 
                "request_id":request_id, 
                "message": "success",
                "response": non_llm_response,
                "gen_file_url_list": gen_file_url_list,
                "qa_type": qa_type,
                "func_name":func_name,
                "func_params":func_params,
                "thought_inference":thought_inference,
                "history":history_tmp,                
                "finish": finish_reason,
                "usage": usage,
                "model":""
            }
            if need_search_list:
                result["search_list"] = search_list
            
            logger.info(f'{request_id} {query[:QUERY_SHORT]}--->generate_stream_non_llm_answer:{non_llm_response}')
            logger.info(f'{request_id} {query[:QUERY_SHORT]}--->generate_stream_non_llm_response:{json.dumps(result, ensure_ascii=False)}')    
            
            result["history"] = history_tmp
            
             
            
            jsonarr = json.dumps(result, ensure_ascii=False)
            id += 1
            str_out = f'id:{id}\nevent:result\ndata:{jsonarr}\n\n'
                   
    
            yield str_out

        except GeneratorExit as e:
            logger.info(f"{request_id} {query[:QUERY_SHORT]}--->GeneratorExit:{str(e)}")
            logger.info(f"{request_id} {query[:QUERY_SHORT]}--->The request is terminated:{non_llm_response}") 
            result = {
                "code": 0,
                "agent_id":agent_id, 
                "session_id":session_id, 
                "request_id":request_id,                            
                "message": "The request is terminated.",
                "response": "",
                "gen_file_url_list": [],
                "qa_type": qa_type,
                "func_name":"",
                "func_params":"",
                "thought_inference":"",
                "history": [],
                "finish": 3,
                "usage": {}
            }
        
            if need_search_list:
                result["search_list"] = search_list
        
            jsonarr = json.dumps(result, ensure_ascii=False)
            id += 1
            str_out = f'id:{id}\nevent:result\ndata:{jsonarr}\n\n'
            logger.info(f'{request_id} {query[:QUERY_SHORT]}--->generate_stream_response:{jsonarr}')  
            yield str_out
             
        finally:            
            logger.info(f"{request_id} {query[:QUERY_SHORT]}--->stream response end")   

    
        
def generate_non_stream_response(query, rewrite_query, prompt, llm_response, qa_type, history, 
                                 ignore_in_history=False, need_search_list=False, search_list=[], 
                                 gen_file_url_list=[], upload_file_url='', non_llm_response=None, 
                                 func_name='', func_params='', thought_inference='', request_id="", 
                                 session_id="", agent_id="",resourceId = ""):
    """
    非流式输出。
    :param llm_response: 来自大模型的 response，可能是 AIMessage 或 HTTP 错误
    :param non_llm_response: 非来自大模型的 response，目前仅用于文生图场景
    """

    # 直接在历史记录中添加新的项
    history_tmp = history[-Default_HISTORY_NUMS:].copy() if history else []
    subjson = {}

    logger.info(f"llm_response:{llm_response}")

    # **1. 处理 AIMessage 响应**
    if isinstance(llm_response, AIMessage):
        if qa_type in [qa_types["ACTION"], qa_types["FUNC_CALL"]]:
            thought_inference = f"{thought_inference}\n{llm_response.content}"
            
        if not ignore_in_history:
            subjson = {
                "session_id": session_id,
                "request_id": request_id,
                "query": query,
                "rewrite_query": rewrite_query,
                "upload_file_url": upload_file_url,
                "response": clean_response_content(llm_response.content),
                "gen_file_url_list": gen_file_url_list,
                "qa_type": qa_type,
                "func_name": func_name,
                "func_params": func_params,
                "thought_inference": thought_inference
            }
            history_tmp.append(subjson)

            #if session_id:
                #redis_client.set_value(session_id, {"role": "user", "content": prompt})
                #redis_client.set_value(session_id, {"role": "assistant", "content": subjson["response"]})
                #redis_client.set_value(f"{session_id}_requery", subjson)

        usage = {
            "prompt_tokens": llm_response.usage_metadata.get("input_tokens", 0),
            "completion_tokens": llm_response.usage_metadata.get("output_tokens", 0),
            "total_tokens": llm_response.usage_metadata.get("total_tokens", 0)
        }

        model = llm_response.response_metadata.get("model_name", "")

        result = {
            "code": 0,
            "session_id": session_id,
            "request_id": request_id,
            "message": "success",
            "response": llm_response.content,
            "gen_file_url_list": gen_file_url_list,
            "qa_type": qa_type,
            "func_name": func_name,
            "func_params": func_params,
            "thought_inference": thought_inference,
            "history": history_tmp,
            "usage": usage,
            "model": model
        }

        if need_search_list:
            result["search_list"] = search_list

        jsonarr = json.dumps(result, ensure_ascii=False)
        logger.info(f'{request_id} {query[:QUERY_SHORT]}--->generate_non_stream_response: {jsonarr}')

        return JsonResponse(result, json_dumps_params={'ensure_ascii': False})

    # **2. 处理非 LLM 响应**
    elif non_llm_response or non_llm_response == '':
        if qa_type in [qa_types["ACTION"], qa_types["FUNC_CALL"]]:
            thought_inference = f"{thought_inference}\n{non_llm_response}"

        if not ignore_in_history:
            subjson = {
                "session_id": session_id,
                "request_id": request_id,
                "query": query,
                "rewrite_query": rewrite_query,
                "upload_file_url": upload_file_url,
                "response": clean_response_content(non_llm_response),
                "gen_file_url_list": gen_file_url_list,
                "qa_type": qa_type,
                "func_name": func_name,
                "func_params": func_params,
                "thought_inference": thought_inference
            }
            history_tmp.append(subjson)

            # if session_id:
                #redis_client.set_value(session_id, {"role": "user", "content": prompt})
                #redis_client.set_value(session_id, {"role": "assistant", "content": subjson["response"]})
                #redis_client.set_value(f"{session_id}_requery", subjson)

        usage = {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}

        result = {
            "code": 0,
            "agent_id": agent_id,
            "session_id": session_id,
            "request_id": request_id,
            "message": "success",
            "response": non_llm_response,
            "gen_file_url_list": gen_file_url_list,
            "qa_type": qa_type,
            "func_name": func_name,
            "func_params": func_params,
            "thought_inference": thought_inference,
            "history": history_tmp,
            "usage": usage,
            "model": ""
        }

        if need_search_list:
            result["search_list"] = search_list

        jsonarr = json.dumps(result, ensure_ascii=False)
        logger.info(f'{request_id} {query[:QUERY_SHORT]}--->generate_non_stream_response: {jsonarr}')

        return JsonResponse(result, json_dumps_params={'ensure_ascii': False})

    # **3. 处理 HTTP 异常**
    else:
        msg_error = "当前大模型服务繁忙或不可用，请稍后重试~"
        
        error_dict = extract_json(llm_response)
        # print(type(error_dict))
        # print(error_dict)

        msg_error = error_dict.get("msg", msg_error)

        logger.info(f"{request_id} {query[:QUERY_SHORT]}--->{msg_error}--->具体报错：{llm_response}")

        result = {
            "code": 0,
            "agent_id": agent_id,
            "session_id": session_id,
            "request_id": request_id,
            "message": "failed",
            "response": msg_error,
            "gen_file_url_list": [],
            "qa_type": qa_type,
            "func_name": "",
            "func_params": "",
            "thought_inference": "",
            "history": [],
            "finish": 3,
            "usage": {}
        }

        if need_search_list:
            result["search_list"] = search_list

        jsonarr = json.dumps(result, ensure_ascii=False)
        logger.info(f'{request_id} {query[:QUERY_SHORT]}--->generate_non_stream_response: {jsonarr}')

        return JsonResponse(result, json_dumps_params={'ensure_ascii': False})