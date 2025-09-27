import os

import requests

from configs.config_parser import Config

config = Config('./configs/config.ini')

# default
USE_DATA_FLYWHEEL = config.getboolean('DEFAULT', 'USE_DATA_FLYWHEEL')
USE_POST_FILTER = config.getboolean('DEFAULT', 'USE_POST_FILTER')
TIME_OUT = config.getint('DEFAULT', 'TIME_OUT')
SSE_USE_MONGO = config.getboolean('DEFAULT', 'SSE_USE_MONGO')

# kafka
KAFKA_BOOTSTRAP_SERVERS = [os.getenv("KAFKA_BOOTSTRAP_SERVERS")]
KAFKA_SASL_PLAIN_USERNAME = os.getenv("KAFKA_SASL_PLAIN_USERNAME")
KAFKA_SASL_PLAIN_PASSWORD = os.getenv("KAFKA_SASL_PLAIN_PASSWORD")
if KAFKA_BOOTSTRAP_SERVERS is None or KAFKA_SASL_PLAIN_USERNAME is None or KAFKA_SASL_PLAIN_PASSWORD is None:
    KAFKA_BOOTSTRAP_SERVERS = config.getlist('KAFKA', 'BOOTSTRAP_SERVERS')
    KAFKA_SASL_PLAIN_USERNAME = config.getstr('KAFKA', 'SASL_PLAIN_USERNAME')
    KAFKA_SASL_PLAIN_PASSWORD = config.getstr('KAFKA', 'SASL_PLAIN_PASSWORD')
KAFKA_USE_ASYN_ADD = config.getboolean('KAFKA', 'USE_ASYN_ADD')
KAFKA_SASL_USE = config.getboolean('KAFKA', 'SASL_USE')
KAFKA_TOPICS = config.getstr('KAFKA', 'TOPICS')
KAFKA_GROUP_ID = config.getstr('KAFKA', 'GROUP_ID')
KAFKA_ENABLE_AUTO_COMMIT = config.getboolean('KAFKA', 'ENABLE_AUTO_COMMIT')
FLYWHEEL_KAFKA_TOPIC = config.getstr('KAFKA', 'FLYWHEEL_KAFKA_TOPIC')

MQ_REL_URL = os.getenv("KAFKA_MQ_REL_URL")
if MQ_REL_URL is None:
    MQ_REL_URL = config.getstr('KAFKA', 'MQ_REL_URL')
MQ_URL_URL = config.getstr('KAFKA', 'MQ_URL_URL')
MQ_URLINSERT_URL = config.getstr('KAFKA', 'MQ_URLINSERT_URL')

DOC_STATUS_INIT_URL = os.getenv("KAFKA_DOC_STATUS_INIT_URL")
if DOC_STATUS_INIT_URL is None:
    DOC_STATUS_INIT_URL = config.getstr('KAFKA', 'DOC_STATUS_INIT_URL')


# object storage
USE_OSS = config.getboolean('OSS', 'USE_OSS')
if USE_OSS:  # 获取配置文件中的桶名
    BUCKET_NAME = config.getstr('OSS', 'OSS_BUCKET_NAME')
else:
    BUCKET_NAME = config.getstr('MINIO', 'MINIO_BUCKET_NAME')

MINIO_ADDRESS = os.getenv("MINIO_ADDRESS")
MINIO_ACCESS_KEY = os.getenv("MINIO_ACCESS_KEY")
MINIO_SECRET_KEY = os.getenv("MINIO_SECRET_KEY")
if MINIO_ADDRESS is None or MINIO_ACCESS_KEY is None or MINIO_SECRET_KEY is None:
    MINIO_ADDRESS = config.getstr('MINIO', 'MINIO_ADDRESS')
    MINIO_ACCESS_KEY = config.getstr('MINIO', 'MINIO_ACCESS_KEY')
    MINIO_SECRET_KEY = config.getstr('MINIO', 'MINIO_SECRET_KEY')

MINIO_UPLOAD_BUCKET_NAME = config.getstr('MINIO', 'MINIO_UPLOAD_BUCKET_NAME')
SECURE = config.getboolean('MINIO', 'SECURE')
REPLACE_MINIO_DOWNLOAD_URL = os.getenv("REPLACE_MINIO_DOWNLOAD_URL")
if REPLACE_MINIO_DOWNLOAD_URL is None:
    REPLACE_MINIO_DOWNLOAD_URL = config.getstr('MINIO', 'REPLACE_MINIO_DOWNLOAD_URL')


# redis
REDIS_ADDRESS = os.getenv("REDIS_HOST")
REDIS_PORT = os.getenv("REDIS_PORT")
REDIS_PASSWD = os.getenv("REDIS_PASSWD")
REDIS_DB = config.getstr('REDIS', 'REDIS_DB')
if REDIS_ADDRESS is None or REDIS_PORT is None or REDIS_PASSWD is None:
    REDIS_ADDRESS = config.getstr('REDIS', 'REDIS_HOST')
    REDIS_PORT = config.getstr('REDIS', 'REDIS_PORT')
    REDIS_PASSWD = config.getstr('REDIS', 'REDIS_PASSWD')


# mongo
MONGO_URL = os.getenv("MONGO_URL")
if MONGO_URL is None:
    MONGO_URL = config.getstr('MONGO', 'MONGO_URL')


# llm
TEMPERATURE = config.getfloat('LLM', 'TEMPERATURE')


# rerank
TRUNCATE_PROMPT = config.getboolean('LLM', 'TRUNCATE_PROMPT')
CONTEXT_LENGTH = config.getint('LLM', 'CONTEXT_LENGTH')


# Milvus wrap server
MILVUS_BASE_URL = config.getstr('MILVUS', 'MILVUS_URL')

# ES wrap server
ES_BASE_URL = config.getstr('ES', 'ES_URL')


#model
MODEL_PROVIDER_URL = os.getenv("MODEL_PROVIDER_URL")
MODEL_PROVIDER_ACCESS_TOKEN = ""
if MODEL_PROVIDER_URL is None:
    MODEL_PROVIDER_URL = config.getstr('MODEL_PROVIDER', 'MODEL_PROVIDER_URL')
    MODEL_PROVIDER_ACCESS_TOKEN = config.getstr('MODEL_PROVIDER', 'MODEL_PROVIDER_ACCESS_TOKEN')

