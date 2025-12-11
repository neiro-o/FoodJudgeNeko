#!/usr/bin/env python3
"""
最小集 Elasticsearch 连接测试脚本
测试 ES 连接，10 秒超时
"""

import sys
import yaml
from pathlib import Path
from elasticsearch import Elasticsearch
from elasticsearch.exceptions import ConnectionError, TransportError, RequestError

def test_es_connection(config_path: str = "../config.yml", timeout: int = 10):
    """
    测试 Elasticsearch 连接
    
    Args:
        config_path: 配置文件路径
        timeout: 连接超时时间（秒）
    """
    try:
        # 加载配置
        config_file = Path(__file__).parent / config_path
        with open(config_file, 'r', encoding='utf-8') as f:
            config = yaml.safe_load(f)
        
        es_config = config['elasticsearch']
        
        # 构建 ES 客户端配置
        es_client_config = {
            'hosts': es_config['hosts'],
            'timeout': timeout,  # 10 秒超时
        }
        
        # 添加认证（如果提供）
        if es_config.get('username') and es_config.get('password'):
            es_client_config['basic_auth'] = (es_config['username'], es_config['password'])
        
        # 配置 SSL（如果是 HTTPS）
        use_https = any(host.startswith('https://') for host in es_config['hosts'])
        if use_https:
            es_client_config['verify_certs'] = False
            es_client_config['ssl_show_warn'] = False
        
        # 创建客户端
        es_client = Elasticsearch(**es_client_config)
        
        # 测试连接
        print(f"正在测试 Elasticsearch 连接...")
        print(f"主机: {es_config['hosts']}")
        print(f"超时: {timeout} 秒")
        print("-" * 50)
        
        if es_client.ping():
            # 获取集群信息
            info = es_client.info()
            print("✅ 连接成功！")
            print(f"集群名称: {info['cluster_name']}")
            print(f"ES 版本: {info['version']['number']}")
            return True
        else:
            print("❌ 连接失败：ping 未响应")
            return False
            
    except ConnectionError as e:
        print(f"❌ 连接错误: {e}")
        return False
    except TransportError as e:
        print(f"❌ 传输错误: {e}")
        return False
    except RequestError as e:
        print(f"❌ 请求错误: {e}")
        return False
    except FileNotFoundError:
        print(f"❌ 配置文件未找到: {config_file}")
        return False
    except Exception as e:
        print(f"❌ 未知错误: {e}")
        return False
    finally:
        if 'es_client' in locals():
            es_client.close()

if __name__ == "__main__":
    success = test_es_connection(timeout=10)
    sys.exit(0 if success else 1)
