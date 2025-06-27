import requests
import json
import re
from utils.uni_id import generate_unique_id
from utils.timing import advanced_timing_decorator
import configparser

config = configparser.ConfigParser()
config.read('config.ini', encoding='utf-8')

BOCHA_SUBSCRIPTION_KEY = config["BING"]["BOCHA_SUBSCRIPTION_KEY"]
BOCHA_SEARCH_URL = config["BING"]["BOCHA_SEARCH_URL"]

def clean_text(text):
    """清除文本中的特殊字符和多余的空白，以及HTML标签。"""
    patterns = [
        r'\xa0+', r'\u3000', r'\t+', r'\r+', r'\n+',   # 清除特殊空白字符和多行换行符
        r'<[/]?b>|&gt;|&lt;'                        # 清除HTML标签
    ]
    for pattern in patterns:
        text = re.sub(pattern, '', text)
    return text.strip()

@advanced_timing_decorator(task_name="bocha_cleaned_search")
def bocha_cleaned_search(query, result_len):
    url = BOCHA_SEARCH_URL

    payload = json.dumps({
        "query": query,
        "summary": True,
        "count": result_len,
        "page": 1
    })

    headers = {
        'Authorization': BOCHA_SUBSCRIPTION_KEY,  # 请替换为你自己的API密钥
        'Content-Type': 'application/json'
    }

    try:
        # 发起请求并检查状态码
        response = requests.request("POST", url, headers=headers, data=payload)
        response.raise_for_status()  # 如果返回的状态码不是 2xx，会抛出异常
        response_data = response.json()  # 尝试解析 JSON
        
        # print("返回数据:", response_data)
        # print()

        # 映射返回结果
        if 'data' in response_data and 'webPages' in response_data['data']:
            results = response_data['data']['webPages']['value']
            mapped_results = [
                {
                    "type": "SE",
                    "id": item.get("id", ""),
                    "title": clean_text(item.get("name", "")),
                    "snippet": clean_text(item.get("snippet", "")),
                    "link": item.get("url", ""),
                    "datePublished": item.get("datePublished", ""),
                    "dateLastCrawled": item.get("dateLastCrawled", "")
                }
                for item in results
            ]
            return mapped_results
        else:
            print("返回数据中没有 webPages 数据")
            return []
    
    except requests.exceptions.RequestException as req_err:
        print(f"请求发生错误: {req_err}")
        return []

    except json.JSONDecodeError as json_err:
        print(f"JSON 解码错误: {json_err}")
        return []

    except KeyError as key_err:
        print(f"缺少预期的键: {key_err}")
        return []

    except Exception as err:
        # 特别处理不同的 HTTP 错误码
        if response.status_code == 403:
            print("错误 403: 余额不足。请前往 https://open.bochaai.com 进行充值")
        elif response.status_code == 400:
            error_message = response.json().get("message", "")
            if "Missing parameter query" in error_message:
                print("错误 400: 请求参数缺失 - 缺少查询参数（query）")
            elif "The API KEY is missing" in error_message:
                print("错误 400: 权限校验失败 - Header 缺少 Authorization")
        elif response.status_code == 401:
            print("错误 401: API KEY 无效，权限校验失败")
        elif response.status_code == 429:
            print("错误 429: 请求频率达到限制，请稍后再试，具体限制详见 API 定价")
        elif response.status_code == 500:
            print("错误 500: 服务器内部错误，请稍后重试")
        else:
            print(f"未知错误发生: {err}")

        return []


if __name__ == '__main__':
    query = "联通的王利民去中国移动了吗"
    result_len = 5
    result = bocha_cleaned_search(query, result_len)
    print(result)