#!/usr/bin/env python3
"""
Elasticsearch initialization functions for mtv2 backend.
Creates index with proper mappings for ES8 with IK Analyzer.
"""

import urllib3
from elasticsearch import Elasticsearch
from elasticsearch.exceptions import RequestError, ConnectionError, SSLError, AuthenticationException

# Disable SSL warnings for development
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)


def check_ik_analyzer(es: Elasticsearch) -> bool:
    """Check if IK Analyzer plugin is installed"""
    try:
        # Try to use IK analyzer in a test request
        es.indices.analyze(body={
            "analyzer": "ik_max_word",
            "text": "测试"
        })
        return True
    except:
        return False


def print_ik_installation_instructions():
    """Print IK Analyzer installation instructions for ES8"""
    print("\n" + "=" * 60)
    print("IK Analyzer Plugin Not Found")
    print("=" * 60)
    print("\nTo install IK Analyzer for Elasticsearch 8.x:")
    print("\n1. Find your Elasticsearch version:")
    print("   curl http://localhost:9200")
    print("\n2. Download IK Analyzer plugin:")
    print("   # For ES 8.11.0:")
    print("   wget https://github.com/medcl/elasticsearch-analysis-ik/releases/download/v8.11.0/elasticsearch-analysis-ik-8.11.0.zip")
    print("\n   # Or for other versions, check:")
    print("   https://github.com/medcl/elasticsearch-analysis-ik/releases")
    print("\n3. Install the plugin:")
    print("   cd /path/to/elasticsearch")
    print("   bin/elasticsearch-plugin install file:///path/to/elasticsearch-analysis-ik-8.x.x.zip")
    print("\n4. Restart Elasticsearch")
    print("\n5. Verify installation:")
    print("   curl -X GET 'http://localhost:9200/_cat/plugins'")
    print("\n" + "=" * 60 + "\n")


def get_es_mapping():
    """Get Elasticsearch index mapping with IK Analyzer for Chinese text"""
    return {
        "mappings": {
            "properties": {
                "id": {"type": "keyword"},
                "mongo_id": {"type": "keyword"},
                "user_review": {
                    "type": "text",
                    "analyzer": "ik_max_word",
                    "search_analyzer": "ik_smart"
                },
                "searchable_content": {
                    "type": "text",
                    "analyzer": "ik_max_word",
                    "search_analyzer": "ik_smart"
                },
                "review_pics": {"type": "keyword"},
                "timestamp": {"type": "long"},
                "others": {
                    "type": "text",
                    "analyzer": "ik_max_word",
                    "search_analyzer": "ik_smart"
                },
                "problem_type": {"type": "integer"},
                "answer": {"type": "integer"},
                "ratio_1": {"type": "float"},
                "ratio_2": {"type": "float"},
                "uploader": {"type": "keyword"},
                "taskId": {"type": "keyword"},
                "userId": {"type": "keyword"},
                "created_at": {"type": "long"},
                "replies": {
                    "type": "nested",
                    "properties": {
                        "role": {"type": "keyword"},
                        "timestamp": {"type": "long"},
                        "content": {
                            "type": "text",
                            "analyzer": "ik_max_word",
                            "search_analyzer": "ik_smart"
                        }
                    }
                },
                "appeals": {
                    "type": "nested",
                    "properties": {
                        "role": {"type": "keyword"},
                        "timestamp": {"type": "long"},
                        "content": {
                            "type": "text",
                            "analyzer": "ik_max_word",
                            "search_analyzer": "ik_smart"
                        },
                        "pics": {"type": "keyword"}
                    }
                },
                "order_info": {
                    "type": "text",
                    "analyzer": "ik_max_word",
                    "search_analyzer": "ik_smart"
                },
                "orders": {
                    "type": "nested",
                    "properties": {
                        "name": {
                            "type": "text",
                            "analyzer": "ik_max_word",
                            "search_analyzer": "ik_smart"
                        },
                        "count": {"type": "integer"},
                        "desc": {
                            "type": "text",
                            "analyzer": "ik_max_word",
                            "search_analyzer": "ik_smart"
                        },
                        "selection": {"type": "keyword"},
                        "pic": {"type": "keyword"},
                        "others": {
                            "type": "text",
                            "analyzer": "ik_max_word",
                            "search_analyzer": "ik_smart"
                        }
                    }
                },
                "order_detail": {
                    "type": "object",
                    "properties": {
                        "order_started": {"type": "long"},
                        "order_finished": {"type": "long"},
                        "deliver_time": {"type": "long"},
                        "total_time": {"type": "long"},
                        "deliver_by": {"type": "keyword"},
                        "note": {
                            "type": "text",
                            "analyzer": "ik_max_word",
                            "search_analyzer": "ik_smart"
                        },
                        "utensils": {"type": "integer"},
                        "invoice": {"type": "boolean"}
                    }
                },
                "comments": {
                    "type": "nested",
                    "properties": {
                        "userid": {"type": "long"},
                        "name": {"type": "keyword"},
                        "content": {
                            "type": "text",
                            "analyzer": "ik_max_word",
                            "search_analyzer": "ik_smart"
                        },
                        "timestamp": {"type": "long"},
                        "choice": {"type": "integer"}
                    }
                }
            }
        },
        "settings": {
            "number_of_shards": 1,
            "number_of_replicas": 0
        }
    }


def create_index(es: Elasticsearch, index_name: str, recreate=False):
    """Create Elasticsearch index with proper mapping"""
    mapping = get_es_mapping()
    
    print(f"Creating Elasticsearch index '{index_name}'...")
    try:
        if es.indices.exists(index=index_name):
            if recreate:
                es.indices.delete(index=index_name)
                print(f"✓ Deleted existing index '{index_name}'")
            else:
                print(f"⚠ Index '{index_name}' already exists. Skipping creation.")
                return
        
        es.indices.create(index=index_name, body=mapping)
        print(f"✓ Index '{index_name}' created successfully")
    except RequestError as e:
        if "resource_already_exists_exception" in str(e):
            print(f"⚠ Index '{index_name}' already exists")
        else:
            print(f"✗ Failed to create index: {e}")
            raise
    except Exception as e:
        print(f"✗ Failed to create index: {e}")
        raise


def init_elasticsearch(config, recreate_index=False):
    """Initialize Elasticsearch: check IK plugin and create index"""
    hosts = config.get('elasticsearch', {}).get('hosts', ['http://localhost:9200'])
    index_name = config.get('elasticsearch', {}).get('index_name', 'problems')
    username = config.get('elasticsearch', {}).get('username', '')
    password = config.get('elasticsearch', {}).get('password', '')
    
    print(f"Connecting to Elasticsearch at {hosts}...")
    try:
        es_config = {
            'hosts': hosts,
            'request_timeout': 30  # 30 second timeout for requests
        }
        if username and password:
            es_config['basic_auth'] = (username, password)
        
        # Disable SSL verification for HTTPS connections (development only)
        # Check if any host uses HTTPS
        use_https = any(host.startswith('https://') for host in hosts)
        if use_https:
            es_config['verify_certs'] = False
            es_config['ssl_show_warn'] = False
        
        es = Elasticsearch(**es_config)
        
        # Try to ping with better error handling
        try:
            if not es.ping():
                raise Exception("Elasticsearch ping returned False")
        except Exception as ping_error:
            # Try to get more info by attempting info() call
            try:
                es.info()
            except Exception as info_error:
                raise Exception(f"Failed to ping Elasticsearch: {ping_error}. Info error: {info_error}")
            raise Exception(f"Failed to ping Elasticsearch: {ping_error}")
        
        cluster_info = es.info()
        es_version = cluster_info.get('version', {}).get('number', 'unknown')
        print(f"✓ Connected to Elasticsearch {es_version}")
        
        # Check IK Analyzer
        print("\nChecking IK Analyzer plugin...")
        if check_ik_analyzer(es):
            print("✓ IK Analyzer plugin is installed")
        else:
            print("✗ IK Analyzer plugin is NOT installed")
            print_ik_installation_instructions()
            raise Exception("IK Analyzer plugin is required but not installed")
        
    except ConnectionError as e:
        print(f"✗ Connection error: {e}")
        print("  → Check if Elasticsearch is running and accessible")
        print(f"  → Hosts: {hosts}")
        raise
    except SSLError as e:
        print(f"✗ SSL/TLS error: {e}")
        print("  → SSL verification is disabled, but connection still failed")
        print("  → Check if Elasticsearch is configured for HTTPS")
        raise
    except AuthenticationException as e:
        print(f"✗ Authentication error: {e}")
        print("  → Check username and password in config.yml")
        raise
    except Exception as e:
        if "IK Analyzer plugin is required" in str(e):
            raise
        print(f"✗ Failed to connect to Elasticsearch: {e}")
        print(f"  → Error type: {type(e).__name__}")
        raise
    
    print()
    create_index(es, index_name, recreate=recreate_index)
    print()
    
    return es
