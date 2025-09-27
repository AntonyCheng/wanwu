from settings import ES_HOSTS, ES_USER, ES_PASSWORD, ES_VERIFY_CERTS
from elasticsearch import Elasticsearch

# 连接到Elasticsearch远程实例
es = Elasticsearch(
    hosts=ES_HOSTS,
    basic_auth=(ES_USER, ES_PASSWORD),
    verify_certs=ES_VERIFY_CERTS,  # 生产环境中应该设为True，以验证SSL证书
    timeout=60,
    retry_on_timeout=True,
)

def get_health():
    # 获取集群健康状态，其中包含节点数量信息
    cluster_health = es.cluster.health()
    return cluster_health.body
