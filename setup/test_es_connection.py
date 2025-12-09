#!/usr/bin/env python3
"""
Test script to debug Elasticsearch connection issues.
"""

import sys
import yaml
import urllib3
from pathlib import Path
from elasticsearch import Elasticsearch
from elasticsearch.exceptions import ConnectionError, SSLError, AuthenticationException

# Disable SSL warnings for development
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

def test_connection():
    """Test Elasticsearch connection with detailed error reporting"""
    config_path = Path(__file__).parent.parent / "config.yml"
    
    try:
        with open(config_path, 'r') as f:
            config = yaml.safe_load(f)
    except Exception as e:
        print(f"✗ Failed to load config: {e}")
        return False
    
    hosts = config.get('elasticsearch', {}).get('hosts', ['http://localhost:9200'])
    username = config.get('elasticsearch', {}).get('username', '')
    password = config.get('elasticsearch', {}).get('password', '')
    
    print("=" * 60)
    print("Elasticsearch Connection Test")
    print("=" * 60)
    print(f"Hosts: {hosts}")
    print(f"Username: {username if username else '(none)'}")
    print(f"Password: {'*' * len(password) if password else '(none)'}")
    print()
    
    # Test 1: Basic connection
    print("Test 1: Creating Elasticsearch client...")
    try:
        es_config = {
            'hosts': hosts,
            'request_timeout': 30  # 30 second timeout for requests
        }
        
        if username and password:
            es_config['basic_auth'] = (username, password)
        
        use_https = any(host.startswith('https://') for host in hosts)
        if use_https:
            print("  → HTTPS detected, disabling SSL verification...")
            es_config['verify_certs'] = False
            es_config['ssl_show_warn'] = False
            es_config['use_ssl'] = True
        
        es = Elasticsearch(**es_config)
        print("  ✓ Client created successfully")
    except Exception as e:
        print(f"  ✗ Failed to create client: {e}")
        return False
    
    # Test 2: Ping
    print("\nTest 2: Testing ping...")
    try:
        result = es.ping()
        if result:
            print("  ✓ Ping successful")
        else:
            print("  ✗ Ping returned False")
            return False
    except ConnectionError as e:
        print(f"  ✗ Connection error: {e}")
        print(f"     Details: {type(e).__name__}")
        return False
    except SSLError as e:
        print(f"  ✗ SSL error: {e}")
        print(f"     Details: {type(e).__name__}")
        return False
    except AuthenticationException as e:
        print(f"  ✗ Authentication error: {e}")
        print(f"     Details: {type(e).__name__}")
        return False
    except Exception as e:
        print(f"  ✗ Unexpected error: {e}")
        print(f"     Type: {type(e).__name__}")
        import traceback
        traceback.print_exc()
        return False
    
    # Test 3: Get cluster info
    print("\nTest 3: Getting cluster info...")
    try:
        info = es.info()
        version = info.get('version', {}).get('number', 'unknown')
        cluster_name = info.get('cluster_name', 'unknown')
        print(f"  ✓ Cluster: {cluster_name}")
        print(f"  ✓ Version: {version}")
    except Exception as e:
        print(f"  ✗ Failed to get info: {e}")
        return False
    
    # Test 4: Check IK Analyzer
    print("\nTest 4: Checking IK Analyzer plugin...")
    try:
        result = es.indices.analyze(body={
            "analyzer": "ik_max_word",
            "text": "测试"
        })
        print("  ✓ IK Analyzer is installed and working")
    except Exception as e:
        print(f"  ✗ IK Analyzer test failed: {e}")
        print("     (This might be okay if plugin is not installed yet)")
    
    print("\n" + "=" * 60)
    print("✓ All connection tests passed!")
    print("=" * 60)
    return True

if __name__ == "__main__":
    success = test_connection()
    sys.exit(0 if success else 1)
