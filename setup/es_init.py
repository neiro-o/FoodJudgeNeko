#!/usr/bin/env python3
"""
Elasticsearch initialization functions for mtv2 backend.
Creates index with proper mappings for ES8 with N-gram analyzer for Chinese text.
Uses character-level n-gram indexing for OCR robustness (handles typos, missing chars, order issues).
"""

import urllib3
from elasticsearch import Elasticsearch
from elasticsearch.exceptions import RequestError, ConnectionError, SSLError, AuthenticationException

# Disable SSL warnings for development
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)


def get_es_mapping():
    """
    Get Elasticsearch index mapping with N-gram analyzer for Chinese text.
    
    Uses character-level n-gram (2-3 chars) for indexing to handle:
    - OCR errors (typos, missing characters)
    - Character order issues
    - Partial matches
    
    Search strategy:
    - Prioritizes exact phrase matches
    - Falls back to n-gram matches for fuzzy/partial queries
    """
    return {
        "settings": {
            "number_of_shards": 1,
            "number_of_replicas": 0,
            "analysis": {
                "analyzer": {
                    "chinese_ngram": {
                        "type": "custom",
                        "tokenizer": "chinese_ngram_tokenizer",
                        "filter": ["lowercase"]
                    },
                    "chinese_search": {
                        "type": "custom",
                        "tokenizer": "chinese_ngram_tokenizer",  # Use same n-gram tokenizer for search
                        "filter": ["lowercase"]
                    }
                },
                "tokenizer": {
                    "chinese_ngram_tokenizer": {
                        "type": "ngram",
                        "min_gram": 2,
                        "max_gram": 3,
                        "token_chars": [
                            "letter",
                            "digit",
                            "punctuation",
                            "symbol"
                        ]
                    }
                }
            }
        },
        "mappings": {
            "properties": {
                "id": {"type": "keyword"},
                "mongo_id": {"type": "keyword"},
                "user_review": {
                    "type": "text",
                    "analyzer": "chinese_ngram",
                    "search_analyzer": "chinese_search",
                    "fields": {
                        "keyword": {
                            "type": "keyword",
                            "ignore_above": 256
                        }
                    }
                },
                "searchable_content": {
                    "type": "text",
                    "analyzer": "chinese_ngram",
                    "search_analyzer": "chinese_search"
                },
                "review_pics": {"type": "keyword"},
                "timestamp": {"type": "long"},
                "others": {
                    "type": "text",
                    "analyzer": "chinese_ngram",
                    "search_analyzer": "chinese_search"
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
                            "analyzer": "chinese_ngram",
                            "search_analyzer": "chinese_search"
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
                            "analyzer": "chinese_ngram",
                            "search_analyzer": "chinese_search"
                        },
                        "pics": {"type": "keyword"}
                    }
                },
                "order_info": {
                    "type": "object",
                    "enabled": True
                },
                "orders": {
                    "type": "nested",
                    "properties": {
                        "name": {
                            "type": "text",
                            "analyzer": "chinese_ngram",
                            "search_analyzer": "chinese_search"
                        },
                        "count": {"type": "integer"},
                        "desc": {
                            "type": "text",
                            "analyzer": "chinese_ngram",
                            "search_analyzer": "chinese_search"
                        },
                        "selection": {"type": "keyword"},
                        "pic": {"type": "keyword"},
                        "others": {
                            "type": "text",
                            "analyzer": "chinese_ngram",
                            "search_analyzer": "chinese_search"
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
                            "analyzer": "chinese_ngram",
                            "search_analyzer": "chinese_search"
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
                            "analyzer": "chinese_ngram",
                            "search_analyzer": "chinese_search"
                        },
                        "timestamp": {"type": "long"},
                        "choice": {"type": "integer"},
                        "likes": {"type": "integer"}
                    }
                }
            }
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
    """Initialize Elasticsearch: create index with N-gram analyzer"""
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
        print("✓ Using N-gram analyzer (no IK Analyzer plugin required)")
        
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
        print(f"✗ Failed to connect to Elasticsearch: {e}")
        print(f"  → Error type: {type(e).__name__}")
        raise
    
    print()
    create_index(es, index_name, recreate=recreate_index)
    print()
    
    return es
