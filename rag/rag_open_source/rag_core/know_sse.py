import os
import json
import ssl
import re
import time
from itertools import product
import shutil
import requests
import numpy as np
from utils.knowledge_base_utils import *
from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse
from sse_starlette.sse import ServerSentEvent, EventSourceResponse
from model_manager import get_model_configure, LlmModelConfig
import urllib3
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

from datetime import datetime, timedelta
from settings import SSE_USE_MONGO, TEMPERATURE, MONGO_URL

from logging_config import setup_logging
logger_name='rag_sse'
app_name = os.getenv("LOG_FILE")
logger = setup_logging(app_name,logger_name)
logger.info(logger_name+'---------LOG_FILE：'+repr(app_name))

from pymongo import MongoClient
from utils import redis_utils
from utils.constant import CHUNK_SIZE
import uuid
import hashlib
user_data_path = r'./user_data'
app = FastAPI()
# 解决跨域问题
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=False,
    allow_methods=["*"],
    allow_headers=["*"]
)
# 初始化 MongoDB 客户端
client = MongoClient(MONGO_URL, 0, connectTimeoutMS=5000, serverSelectionTimeoutMS=3000)

collection = client['rag']['rag_user_logs']
redis_client = redis_utils.get_redis_connection()

def get_query_dict_cache(redis_client, user_id, knowledgebases):
    """
    根据 user_id,查询的知识库knowledgebase列表 查询 Redis 中的缓存，将哈希表字段的值解析为 query_dict。
    :param user_id: 用户ID
    :return: 完整的 query_dict 数据（列表形式），如果缓存不存在则返回 None。
    """
    all_query_dicts = []

    redis_key_list = []
    for knowledgebase in knowledgebases:
        redis_key = f"query_dict:{user_id}:{knowledgebase}"
        redis_key_list.append(redis_key)
    for redis_key in redis_key_list:
        # 获取整个哈希表，返回一个字典，字段是 id，值是对应的条目 JSON 字符串
        term_dict_hash = redis_client.hgetall(redis_key)
        if term_dict_hash:
            # 将每个字段的 JSON 字符串转换为 Python 对象（字典）
            term_dict = [json.loads(value) for value in term_dict_hash.values()]
            all_query_dicts.extend(term_dict)
    # 此处请将all_query_dicts相同元素去重
    # 去重：将所有字典转换为 JSON 字符串，存入集合中，集合自动去重
    unique_query_dicts = {json.dumps(query_dict, sort_keys=True): query_dict for query_dict in all_query_dicts}
    # 返回去重后的字典列表
    return list(unique_query_dicts.values())

def query_rewrite(question, term_dict):
    """
    根据专名同义词表改写用户问题，支持生成多个改写结果（针对多个别名）。

    参数:
    - question (str): 用户输入问题。
    - term_dict (list): 专名同义词表，每项为字典，包含 'name' 和 'alias'。

    返回:
    - list: 改写后的用户问题列表，每个改写对应一种组合方式。
    """
    # 保存所有的替换项
    replacements = []

    for term in term_dict:
        name = term["name"]  # 标准词
        aliases = term["alias"]  # 别名列表

        # 如果问题中包含标准词，则保存替换方案
        if re.search(re.escape(name), question):
            replacements.append([(name, alias) for alias in aliases])

    # 如果没有匹配到标准词，直接返回原问题
    if not replacements:
        return [question]

    # 使用笛卡尔积计算所有可能的替换组合
    combinations = product(*replacements)

    rewritten_questions = []
    for combo in combinations:
        # 逐个应用替换规则
        new_question = question
        for name, alias in combo:
            new_question = re.sub(re.escape(name), alias, new_question)
        rewritten_questions.append(new_question)

    return rewritten_questions


@app.post("/rag/knowledge/stream/search")
async def search(request: Request):
    headers = {
        "Content-Type": "application/json"
    }
    prompt = ''
    history = []
    search_list = []
    async def stream_generate(prompt, history, search_list,question,top_p,repetition_penalty,temperature,custom_model_info,do_sample,score,msg_id):

        answer = ''
        start_time = time.time()
        # llm_url = custom_model_info["model_url"]
        # model_name = custom_model_info["model_name"]
        model_id = custom_model_info["llm_model_id"]
        llm_config = get_model_configure(model_id)
        model_name = llm_config.model_name
        llm_url = ""
        api_key = ""
        if isinstance(llm_config, LlmModelConfig):
            llm_url = llm_config.endpoint_url + "/chat/completions"
            api_key = llm_config.api_key

        headers = {"Content-Type": "application/json", "Authorization": f"Bearer {api_key}"}
        messages = []
        for item in history:
            messages.append({"role": "user", "content": item["query"]})
            messages.append({"role": "assistant", "content": item["response"]})
        messages.append({"role": "user", "content": prompt})
        llm_data = {
            "model": model_name,
            # "pad_token_id": 0,
            # "bos_token_id": 1,
            # "eos_token_id": 2,
            "temperature": temperature,
            # "top_k": 5,
            # "top_p": top_p,
            "repetition_penalty": repetition_penalty,
            "do_sample": do_sample,
            "stream": True,
            "messages": messages,
        }
        response = requests.post(llm_url, json=llm_data, headers=headers, verify=False, stream=True)
        logger.info(f'{llm_url} ====== 大模型开始流式输出，发送到大模型参数：'+repr(llm_data))
        waitting_response = ""
        time_i = 0
        finish = 0
        try:
            id = 0
            for line in response.iter_lines(decode_unicode=True):
                if line.startswith("data:"):
                    line = line[5:]
                    datajson = json.loads(line)
                    yield_time = datetime.now().strftime('%Y-%m-%d %H:%M:%S.%f')
                    # logger.info(f"{yield_time},{datajson}")
                    if "choices" in datajson:
                        #content = datajson["choices"][0]["delta"]["content"]
                        content = datajson.get("choices", [{}])[0].get("delta", {}).get("content", "")
                        #if datajson["choices"][0]["finish_reason"] == "stop":  # 如果模型已经停止输出
                        finish_reason = datajson.get("choices", [{}])[0].get("finish_reason", "")
                        if finish_reason == "stop":
                            finish = 1
                        elif finish_reason == "sensitive_cancel":  # 如果模型已经停止输出
                            finish = 4
                        else:
                            finish = 0
                    else:
                        #content = datajson["data"]["choices"][0]["message"]["content"]
                        content = datajson.get("data", {}).get("choices", [{}])[0].get("message", {}).get("content", "")
                        #if datajson["data"]["choices"][0]["finish_reason"] == "stop":  # 如果模型已经停止输出
                        finish_reason = datajson.get("data", {}).get("choices", [{}])[0].get("finish_reason", "")
                        if finish_reason == "stop":
                            finish = 1
                        elif finish_reason == "sensitive_cancel":  # 如果模型已经停止输出
                            finish = 4
                        else:
                            finish = 0
                    answer += content
                    waitting_response += content
                    history_tmp = history.copy()
                    subjson = {}
                    subjson["query"] = question
                    subjson["response"] = answer
                    subjson["needHistory"] = True
                    history_tmp.append(subjson)
                    response_info = {
                        'code': int(0),
                        "message": "success",
                        "msg_id": msg_id,
                        "data":{"output": content,
                                "searchList": search_list,
                            },
                        "history":history_tmp,
                        "finish": finish
                    }
                    if score != -1:  # 如果允许返回得分
                        response_info["data"]["score"] = score
                    jsonarr = json.dumps(response_info, ensure_ascii=False)
                    id += 1
                    str_out = f'{jsonarr}'
                    yield str_out
                    if time_i == 0:
                        end_time = time.time()
                        logger.info(f"question:{question}。开始流式第一个词返回时间：{end_time - start_time}秒")
                        time_i += 1
                else:  # 适配 openai 的返回格式
                    try:
                        datajson = json.loads(line)
                        if "choices" in datajson:
                            #content = datajson["choices"][0]["delta"]["content"]
                            content = datajson.get("choices", [{}])[0].get("delta", {}).get("content", "")
                            if datajson["choices"][0]["finish_reason"] == "stop":  # 如果模型已经停止输出
                                finish = 1
                            elif datajson["choices"][0]["finish_reason"] == "sensitive_cancel":  # 敏感词
                                finish = 4
                            else:
                                finish = 0
                        else:
                            #content = datajson["data"]["choices"][0]["message"]["content"]
                            content = datajson.get("data", {}).get("choices", [{}])[0].get("message", {}).get("content", "")
                            if datajson["data"]["choices"][0]["finish_reason"] == "stop":  # 如果模型已经停止输出
                                finish = 1
                            elif datajson["data"]["choices"][0]["finish_reason"] == "sensitive_cancel":  # 敏感词
                                finish = 4
                            else:
                                finish = 0
                        answer += content
                        waitting_response += content
                        history_tmp = history.copy()
                        subjson = {}
                        subjson["query"] = question
                        subjson["response"] = answer
                        subjson["needHistory"] = True
                        history_tmp.append(subjson)
                        response_info = {
                            'code': int(0),
                            "message": "success",
                            "msg_id": msg_id,
                            "data": {"output": content,
                                     "searchList": search_list,
                                     },
                            "history": history_tmp,
                            "finish": finish
                        }
                        if score != -1:  # 如果允许返回得分
                            response_info["data"]["score"] = score
                        jsonarr = json.dumps(response_info, ensure_ascii=False)
                        id += 1
                        str_out = f'{jsonarr}'
                        yield str_out
                        if time_i == 0:
                            end_time = time.time()
                            logger.info(f"question:{question}。开始流式第一个词返回时间：{end_time - start_time}秒")
                            time_i += 1
                    except Exception as e:
                        pass
        except Exception as e:  # 如果发生异常，返回错误信息
            logger.error(f"LLM Error url:{llm_url}, err: {e}")
            if finish not in [1, 4]:  # 如果模型没有停止输出，则返回错误信息
                response_info = {
                    'code': 1,
                    "message": f"LLM Error:{e}",
                }
                yield json.dumps(response_info, ensure_ascii=False)
        # ========== 最终流式返回完成后 动作 ===========
        end_time = time.time()
        logger.info(f"question:{question}。流式最后一个词返回时间：{end_time - start_time}秒,返回json:{jsonarr}")
    async def no_search_list(return_answer, history, question, code, msg, score, msg_id):
        answer = ''
        for char in return_answer:
            answer = answer + char

            history_tmp = history.copy()
            subjson = {}
            subjson["query"] = question
            subjson["response"] = answer
            subjson["needHistory"] = True
            history_tmp.append(subjson)

            response_info = {
                'code': code,
                "message": msg,
                "msg_id": msg_id,
                "data": {"output": char,
                         "searchList": [],

                         },
                "history": history_tmp,
                "finish": 0
            }
            if score != -1:  # 如果允许返回得分，返回空
                response_info["data"]["score"] = []
            jsonarr = json.dumps(response_info, ensure_ascii=False)
            str_out = f'{jsonarr}'
            yield str_out
        # ======= 最后返回 ========
        response_info = {
            'code': code,
            "message": msg,
            "msg_id": msg_id,
            "data": {"output": "",
                     "searchList": [],

                     },
            "history": history_tmp,
            "finish": 1
        }
        if score != -1:  # 如果允许返回得分，返回空
            response_info["data"]["score"] = []
        jsonarr = json.dumps(response_info, ensure_ascii=False)
        str_out = f'{jsonarr}'
        yield str_out

    response_info = {
        'code': int(0),
        "message": "success",
        "data": {"output": "",
                 "searchList": [],
                },
        "history":[]

    }


    json_request = await request.json()
    user_id = request.headers.get("X-uid")
    kb_name = json_request["knowledgeBase"]
    question = json_request["question"]
    rate = float(json_request["threshold"])
    top_k = int(json_request["topK"])
    stream = json_request["stream"]
    history = json_request["history"]
    chichat = json_request.get("chichat", True)
    default_answer = json_request.get("default_answer", '根据已知信息，无法回答您的问题。')
    return_meta = json_request.get("return_meta", False)
    prompt_template = json_request.get("prompt_template", '')
    top_p = json_request.get("top_p", 0.85)
    repetition_penalty = json_request.get("repetition_penalty", 1.1)
    temperature = json_request.get("temperature", TEMPERATURE)
    if temperature <= 0.01:  # 强制到0.01以下
        temperature = 0.01
    max_history = json_request.get("max_history", 10)
    custom_model_info = json_request.get("custom_model_info", {})
    search_field = json_request.get('search_field', 'con')

    if "do_sample" not in json_request:  # 如果没有传参，则默认使用temperature决定是否开启采样
        if temperature > 0.1:
            do_sample = True
        else:
            do_sample = False
    else:
        do_sample = json_request.get('do_sample')
    # 是否开启自动引文，此参数与prompt_template互斥，当开启auto_citation时，prompt_template用户传参不生效
    auto_citation = json_request.get("auto_citation", False)
    # 是否开启数据飞轮
    data_flywheel = json_request.get("data_flywheel", False)
    # 是否返回得分
    return_score = json_request.get("return_score", False)
    # 是否query改写
    rewrite_query = json_request.get("rewrite_query", False)
    rerank_mod = json_request.get("rerank_mod", "rerank_model")
    rerank_model_id = json_request.get("rerank_model_id", '')
    weights = json_request.get("weights", None)
    retrieve_method = json_request.get("retrieve_method", "hybrid_search")

    # metadata filtering params
    metadata_filtering = json_request.get("metadata_filtering", False)
    metadata_filtering_conditions = json_request.get("metadata_filtering_conditions", [])
    if not metadata_filtering:
        metadata_filtering_conditions = []

    logger.info('---------------流式查询---------------')
    logger.info('user_id:'+repr(user_id)+'\t'+repr(json_request))

    def params_check_failed(err_msg: str):
        response_info = {
            'code': 1,
            "message": err_msg,
            "data": {"output": "", "searchList": []},
            "history": []
        }
        logger.error(error_msg)
        if json_request.get("stream"):
            return EventSourceResponse(no_search_list(default_answer, history, question, 1, error_msg, -1, ''))
        else:
            return JSONResponse(content=response_info)
    # 检查 custom_model_info['llm_model_id'] 是否为空
    if 'llm_model_id' not in custom_model_info or not custom_model_info.get('llm_model_id'):
        error_msg = "custom_model_info['llm_model_id'] 不能为空"
        return params_check_failed(error_msg)

    # 检查 rerank_model_id 是否为空
    if rerank_mod == "rerank_model" and not rerank_model_id:
        error_msg = "rerank_model_id cannot be empty when using model-based reranking."
        return params_check_failed(error_msg)

    if rerank_mod == "weighted_score" and weights is None:
        error_msg = "weights cannot be empty when using weighted score reranking."
        return params_check_failed(error_msg)
    if weights is not None and not isinstance(weights, dict):
        error_msg = "weights must be a dictionary or None."
        return params_check_failed(error_msg)

    if rerank_mod == "weighted_score" and retrieve_method != "hybrid_search":
        error_msg = "Weighted score reranking is only supported in hybrid search mode."
        return params_check_failed(error_msg)

    sRandom = str(uuid.uuid1()).replace("-", "")
    u_id = "{}_{}_{}_{}".format(user_id, kb_name, question, sRandom)
    msg_id = hashlib.md5(u_id.encode("utf8")).hexdigest()
    chunk_conent=0
    chunk_size=CHUNK_SIZE
    use_cache_flag = False
    if isinstance(kb_name, str):
        kb_names=[kb_name]
    else:
        kb_names=kb_name
    kb_ids = []  # kb_id 的 list
    for kb_n in kb_names:
        kb_ids.append(get_kb_name_id(user_id, kb_n))  # 获取kb_id

    if max_history > 0:
        history = history[-max_history:]
    else:
        history = []
    if rewrite_query:
        query_dict_list = get_query_dict_cache(redis_client,user_id, kb_ids)
        if query_dict_list:
            rewritten_queries = query_rewrite(question, query_dict_list)
            logger.info("对query进行改写,原问题:%s 改写后问题:%s" % (question, ",".join(rewritten_queries)))
            if len(rewritten_queries) > 0:
                question = rewritten_queries[0]
                logger.info("按新问题:%s 进行召回" % question)
        else:
            logger.info("未启用或维护转名词表,query未改写,按原问题:%s 进行召回" % question)
    if top_k<=0:
        # top_k必须大于0
        return EventSourceResponse(no_search_list(default_answer,history,question,1,'top_k必须大于0'))
    else:
        prompt=question
        search_list=[]
        has_effective_cache = False
        try:
            temp_start_time = time.time()
            if data_flywheel:
                # 要存储的数据
                cache_key = "%s^%s^%s^%s" % (user_id, kb_name, top_k, question)
                exists = redis_client.exists(cache_key)
                if exists:
                    use_cache_flag = True
                    logger.info("=========>命中缓存,cache_key=%s" % cache_key)
                    cache_result = redis_client.get(cache_key)
                    # 将字符串转换为 JSON 对象
                    cache_result_json = json.loads(cache_result)
                    if cache_result_json and 'data' in cache_result_json:
                        if 'searchList' in cache_result_json['data'] and 'prompt' in cache_result_json['data'] and 'score' in cache_result_json['data']:
                            if len(cache_result_json["data"]["searchList"]) > 0:
                                has_effective_cache = True
                if has_effective_cache:
                    rerank_result = cache_result_json
                else:
                    rerank_result = get_knowledge_based_answer(user_id, kb_names, question, rate, top_k, chunk_conent,
                                                               chunk_size, return_meta, prompt_template, search_field,
                                                               default_answer, auto_citation, retrieve_method,
                                                               rerank_model_id=rerank_model_id, rerank_mod=rerank_mod,
                                                               weights=weights,
                                                               metadata_filtering_conditions=metadata_filtering_conditions)
            else:
                rerank_result = get_knowledge_based_answer(user_id, kb_names, question, rate, top_k, chunk_conent,
                                                           chunk_size, return_meta, prompt_template, search_field,
                                                           default_answer, auto_citation, retrieve_method,
                                                           rerank_model_id=rerank_model_id, rerank_mod=rerank_mod,
                                                           weights=weights,
                                                           metadata_filtering_conditions=metadata_filtering_conditions)

            logger.info("===>data_flywheel=%s,has_effective_cache=%s,rerank_result=%s" % (data_flywheel,has_effective_cache,json.dumps(rerank_result, ensure_ascii=False)))
            response_info['code'] = int(rerank_result['code'])
            response_info['message'] = str(rerank_result['message'])
            response_info['msg_id'] = str(msg_id)

            search_list = rerank_result['data']['searchList']
            prompt = rerank_result['data']['prompt']
            score = rerank_result['data'].get('score', [])
            logger.info('知识召回结果：'+json.dumps(repr(rerank_result), ensure_ascii=False))
            temp_end_time = time.time()
            logger.info(f"======知识召回使用时间：{temp_end_time - temp_start_time}秒")
        except Exception as e:
            # logger.info('知识召回异常：'+repr(e))
            import traceback
            logger.error("====> 知识召回异常 error %s" % e)
            logger.error(traceback.format_exc())
            response_info['code']=1
            response_info['message']=repr(e)
            response_info['msg_id'] = str(msg_id)

            prompt=question
            search_list=[]
            score = []
        if not return_score:  # 如果不返回分数
            score = -1
        if SSE_USE_MONGO:  # 如果使用mongo
            temp_start_time = time.time()
            message = {"id": msg_id}
            current_date = datetime.now().strftime("%Y%m%d")
            try:
                u_condition = {'id': msg_id}
                message = {
                    "id": msg_id,
                    "user_id": user_id,
                    "kb_name": kb_name,
                    "question": question,
                    "rate": rate,
                    "top_k": top_k,
                    "top_p": top_p,
                    "repetition_penalty": repetition_penalty,
                    "temperature": temperature,
                    "max_history": max_history,
                    "do_sample": do_sample,
                    "return_meta": "true" if return_meta else "false",
                    "auto_citation": "true" if auto_citation else "false",
                    "data_flywheel": "true" if data_flywheel else "false",
                    "return_score": "true" if return_score else "false",
                    "use_cache": "true" if has_effective_cache else "false",
                    "prompt_template": prompt_template,
                    "default_answer": default_answer,
                    "model_name": custom_model_info['llm_model_id'],
                    "search_field": search_field,
                    "search_list": search_list,
                    "scores": score if return_score else [],
                    "status": 0,
                    "update_time": int(round(time.time() * 1000)),
                    "create_time": int(round(time.time() * 1000)),
                    "create_dt": int(current_date)
                }
                # collection.insert_one(message)
                # collection.update_one(u_condition, message, upsert=True)
                collection.update_one(u_condition, {'$set': message}, upsert=True)
                if "_id" in message:
                    del message["_id"]
                logger.info("=======>user log已存储至mongoDB,id=%s,data=%s" % (msg_id, json.dumps(message, ensure_ascii=False)))
            except Exception as err:
                # 存储mongodb异常的时候，接口msg_id返回-1
                msg_id = "-1"
                import traceback
                logger.error("====> stream search save mongoDB error %s" % err)
                logger.error(traceback.format_exc())
        # if not use_cache_flag and data_flywheel:
        #     # 判断在飞轮模式下且若未命中缓存，推送kafka触发飞轮策略构建
        #     try:
        #         kafka_utils.push_kafka_msg(message)
        #         logger.info("=======>user log已推送kakfa")
        #     except Exception as err:
        #         import traceback
        #         logger.error("====> stream search push kafka error %s" % err)
        #         logger.error(traceback.format_exc())
        # 大模型生成返回
            temp_end_time = time.time()
            logger.info(f"======save mongoDB 使用时间：{temp_end_time - temp_start_time}秒")
        if stream:
            if response_info['code'] !=0:
                return EventSourceResponse(no_search_list(default_answer,history,question,response_info['code'],response_info['message'], score, msg_id))
            # 需要大模型输出
            if len(search_list)>0 or chichat:
                return EventSourceResponse(stream_generate(prompt, history, search_list,question,top_p,repetition_penalty,temperature,custom_model_info,do_sample,score,msg_id))
             # 知识召回为空，并且使用兜底话术返回，不需要大模型输出
            else:
                return EventSourceResponse(no_search_list(default_answer,history,question,0,'success', score, msg_id))

        else:  # 非stream返回
            # if response_info['code'] != 0:
            #     response_info = {
            #         'code': response_info['code'],
            #         "message": response_info['message'],
            #         "msg_id": msg_id,
            #         "data": {"output": default_answer,
            #                  "searchList": [],
            #                  },
            #         "history": history
            #     }
            #     if return_score:  # 如果允许返回得分，返回空
            #         response_info["data"]["score"] = []
            #     return JSONResponse(content=response_info)
            # # 需要大模型输出
            # if len(search_list) > 0 or chichat:
            #     response_info = generate(prompt, history, search_list, question, top_p, repetition_penalty, temperature, model_name,do_sample, score,msg_id)
            #     logger.info(f"=======>response_info:{response_info}")
            #     return JSONResponse(content=response_info)
            # # 知识召回为空，并且使用兜底话术返回，不需要大模型输出
            # else:
            #     response_info = {
            #         'code': 0,
            #         "message": "success",
            #         "msg_id": msg_id,
            #         "data": {"output": default_answer,
            #                  "searchList": [],
            #                  },
            #         "history": history
            #     }
            #     if return_score:  # 如果允许返回得分，返回空
            #         response_info["data"]["score"] = []
            #     return JSONResponse(content=response_info)
            response_info = {
                'code': 1,
                "message": "fail",
                "data": {"output": "parameter stream need to be true"}
            }
            return JSONResponse(content=response_info)

