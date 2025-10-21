#!/usr/bin/env python3
"""
Database initialization script
Initializes MongoDB, Elasticsearch, and Redis connections
"""

import pymongo
import redis
import requests
import sys
import yaml
from urllib.parse import urlparse

def read_config(config_path="config.yml"):
    """Read configuration from config.yml"""
    try:
        with open(config_path, 'r', encoding='utf-8') as file:
            config = yaml.safe_load(file)
        return config
    except Exception as e:
        print(f"⚠️  Could not read config.yml: {e}")
        return None

def check_mongodb(uri="mongodb://localhost:27017", database="mt_backend", collections=None):
    """Check MongoDB connection and initialize if needed"""
    try:
        print("🔍 Checking MongoDB connection...")
        client = pymongo.MongoClient(uri, serverSelectionTimeoutMS=5000)
        client.server_info()  # This will raise an exception if connection fails
        print("✅ MongoDB connection successful")
        
        # Use provided database name
        db = client[database]
        
        # Create collections if they don't exist (this is safe - won't clear existing data)
        if collections is None:
            collections = ["users", "invite_codes", "raw_problems", "queue_items"]
        
        for collection_name in collections:
            if collection_name not in db.list_collection_names():
                db.create_collection(collection_name)
                print(f"📁 Created collection: {collection_name}")
            else:
                print(f"📁 Collection exists: {collection_name}")
        
        client.close()
        return True
        
    except Exception as e:
        print(f"❌ MongoDB connection failed: {e}")
        print("💡 Please ensure MongoDB is running on the specified URI")
        return False

def check_chinese_analyzer_plugin(addresses=["http://localhost:9200"]):
    """Check if Chinese analyzer plugin is installed"""
    try:
        for address in addresses:
            try:
                # Check if smartcn plugin is installed
                plugins_response = requests.get(f"{address}/_cat/plugins", timeout=5)
                if plugins_response.status_code == 200:
                    plugins_text = plugins_response.text.lower()
                    if "analysis-smartcn" in plugins_text or "smartcn" in plugins_text:
                        print("✅ Chinese analyzer plugin (analysis-smartcn) is installed")
                        return True
                    else:
                        print("⚠️  Chinese analyzer plugin (analysis-smartcn) is NOT installed")
                        print("💡 To install Chinese language support, run:")
                        print("   sudo bin/elasticsearch-plugin install analysis-smartcn")
                        print("   Then restart Elasticsearch")
                        return False
            except requests.exceptions.RequestException as e:
                print(f"⚠️  Failed to check plugins at {address}: {e}")
                continue
        return False
    except Exception as e:
        print(f"❌ Failed to check Chinese analyzer plugin: {e}")
        return False

def check_elasticsearch(addresses=["http://localhost:9200"]):
    """Check Elasticsearch connection and initialize if needed"""
    try:
        print("🔍 Checking Elasticsearch connection...")
        
        for address in addresses:
            try:
                response = requests.get(f"{address}/_cluster/health", timeout=5)
                if response.status_code == 200:
                    print("✅ Elasticsearch connection successful")
                    
                    # Check for Chinese analyzer plugin
                    check_chinese_analyzer_plugin([address])
                    
                    # Check if problems index exists
                    index_response = requests.get(f"{address}/problems", timeout=5)
                    if index_response.status_code == 404:
                        print("📁 Creating problems index...")
                        # Create index with Chinese language support
                        index_mapping = {
                            "settings": {
                                "analysis": {
                                    "analyzer": {
                                        "chinese_analyzer": {
                                            "type": "custom",
                                            "tokenizer": "smartcn_tokenizer",
                                            "filter": ["lowercase", "stop"]
                                        },
                                        "chinese_search_analyzer": {
                                            "type": "custom",
                                            "tokenizer": "smartcn_tokenizer",
                                            "filter": ["lowercase", "stop"]
                                        }
                                    }
                                }
                            },
                            "mappings": {
                                "properties": {
                                    "id": {"type": "keyword", "index": False},  # Not searched, only for filtering
                                    "user_review": {
                                        "type": "text",
                                        "analyzer": "chinese_analyzer",
                                        "search_analyzer": "chinese_search_analyzer"
                                    },  # SEARCHED with Chinese support
                                    "review_pics": {"type": "keyword", "index": False},  # Not searched
                                    "timestamp": {"type": "long", "index": False},  # Not searched, only for sorting/filtering
                                    "replies": {
                                        "type": "nested",
                                        "properties": {
                                            "role": {"type": "keyword", "index": False},  # Not searched
                                            "timestamp": {"type": "long", "index": False},  # Not searched
                                            "content": {
                                                "type": "text",
                                                "analyzer": "chinese_analyzer",
                                                "search_analyzer": "chinese_search_analyzer"
                                            }  # SEARCHED with Chinese support
                                        }
                                    },
                                    "appeals": {
                                        "type": "nested",
                                        "properties": {
                                            "role": {"type": "keyword", "index": False},  # Not searched
                                            "timestamp": {"type": "long", "index": False},  # Not searched
                                            "content": {
                                                "type": "text",
                                                "analyzer": "chinese_analyzer",
                                                "search_analyzer": "chinese_search_analyzer"
                                            },  # SEARCHED with Chinese support
                                            "pics": {"type": "keyword", "index": False}  # Not searched
                                        }
                                    },
                                    # Optional fields - stored but not indexed for search
                                    "order_info": {"type": "text", "index": False},
                                    "orders": {
                                        "type": "nested",
                                        "enabled": False  # Store but don't index at all
                                    },
                                    "order_detail": {
                                        "type": "object",
                                        "enabled": False  # Store but don't index at all
                                    },
                                    "others": {
                                        "type": "text",
                                        "analyzer": "chinese_analyzer",
                                        "search_analyzer": "chinese_search_analyzer"
                                    }  # SEARCHED with Chinese support
                                }
                            }
                        }
                        
                        create_response = requests.put(
                            f"{address}/problems",
                            json=index_mapping,
                            headers={"Content-Type": "application/json"},
                            timeout=10
                        )
                        
                        if create_response.status_code in [200, 201]:
                            print("📁 Problems index created successfully")
                        else:
                            print(f"⚠️  Index creation response: {create_response.status_code}")
                    else:
                        print("📁 Problems index already exists")
                    
                    return True
                    
            except requests.exceptions.RequestException as e:
                print(f"⚠️  Failed to connect to {address}: {e}")
                continue
        
        print("❌ Could not connect to any Elasticsearch address")
        return False
        
    except Exception as e:
        print(f"❌ Elasticsearch connection failed: {e}")
        print("💡 Please ensure Elasticsearch is running on the specified addresses")
        return False

def check_redis(host="localhost", port=6379, password="", db=0):
    """Check Redis connection"""
    try:
        print("🔍 Checking Redis connection...")
        r = redis.Redis(host=host, port=port, password=password or None, db=db, decode_responses=True)
        r.ping()  # This will raise an exception if connection fails
        print("✅ Redis connection successful")
        return True
        
    except Exception as e:
        print(f"❌ Redis connection failed: {e}")
        print("💡 Please ensure Redis is running on the specified host and port")
        return False

def init_databases(environment="development", config_path="config.yml"):
    """Initialize all databases"""
    print(f"🗄️  Initializing databases for {environment} environment...")
    
    # Read configuration from config.yml
    config = read_config(config_path)
    if config is None:
        print("⚠️  Using default configurations")
        config = {}
    
    # Get configurations from config.yml or use defaults
    mongodb_config = config.get("mongodb", {})
    mongodb_uri = mongodb_config.get("uri", "mongodb://localhost:27017")
    mongodb_database = mongodb_config.get("database", "mt_backend")
    mongodb_collections = list(mongodb_config.get("collections", {}).values())
    
    elasticsearch_config = config.get("elasticsearch", {})
    elasticsearch_addresses = elasticsearch_config.get("addresses", ["http://localhost:9200"])
    
    redis_config = config.get("redis", {})
    redis_host = redis_config.get("host", "localhost")
    redis_port = redis_config.get("port", 6379)
    redis_password = redis_config.get("password", "")
    redis_db = redis_config.get("db", 0)
    
    # Environment-specific configurations could be added here
    if environment == "production":
        print("🏭 Production environment detected - using production database settings")
        # Add production-specific configurations here if needed
    
    # Initialize each database
    success = True
    
    # MongoDB
    if not check_mongodb(mongodb_uri, mongodb_database, mongodb_collections):
        success = False
    
    # Elasticsearch
    if not check_elasticsearch(elasticsearch_addresses):
        success = False
    
    # Redis
    if not check_redis(redis_host, redis_port, redis_password, redis_db):
        success = False
    
    if not success:
        raise Exception("Database initialization failed. Please check the error messages above.")
    
    print("🎉 All databases initialized successfully!")

if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser(description='Initialize databases')
    parser.add_argument('--environment', default='development', 
                       help='Environment (development/production)')
    parser.add_argument('--config', default='config.yml',
                       help='Path to config.yml file')
    args = parser.parse_args()
    
    try:
        init_databases(args.environment, args.config)
    except Exception as e:
        print(f"❌ Database initialization failed: {e}")
        sys.exit(1)
