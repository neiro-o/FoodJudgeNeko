#!/usr/bin/env python3
"""
Get a random document from Elasticsearch

This script retrieves a random document from the problems index in Elasticsearch
and outputs it as JSON.
"""

import sys
import json
import yaml
from pathlib import Path
from typing import Dict, Any

try:
    from elasticsearch import Elasticsearch
    from elasticsearch.exceptions import TransportError, RequestError
except ImportError:
    print("Error: elasticsearch not installed. Run: pip install elasticsearch")
    sys.exit(1)


def load_config(config_path: str = "../config.yml") -> Dict[str, Any]:
    """Load configuration from YAML file."""
    try:
        config_file = Path(__file__).parent / config_path
        with open(config_file, 'r', encoding='utf-8') as f:
            config = yaml.safe_load(f)
        return config
    except Exception as e:
        print(f"Error loading config: {e}")
        sys.exit(1)


def connect_elasticsearch(config: Dict[str, Any]) -> Elasticsearch:
    """Connect to Elasticsearch."""
    try:
        es_config = config['elasticsearch']
        es_client_config = {
            'hosts': es_config['hosts'],
        }
        
        # Only add authentication if both username and password are provided
        if es_config.get('username') and es_config.get('password'):
            es_client_config['basic_auth'] = (es_config['username'], es_config['password'])
        
        # Only configure SSL for HTTPS connections
        use_https = any(host.startswith('https://') for host in es_config['hosts'])
        if use_https:
            es_client_config['verify_certs'] = False
            es_client_config['ssl_show_warn'] = False
        
        es = Elasticsearch(**es_client_config)
        if es.ping():
            return es
        else:
            print("✗ Elasticsearch ping failed")
            sys.exit(1)
    except Exception as e:
        print(f"✗ Elasticsearch connection failed: {e}")
        sys.exit(1)


def get_random_document(es: Elasticsearch, index_name: str) -> Dict[str, Any]:
    """
    Get a random document from Elasticsearch using random_score query.
    
    Args:
        es: Elasticsearch client
        index_name: Index name to search
        
    Returns:
        The document as a dictionary, or None if no documents found
    """
    query = {
        "query": {
            "function_score": {
                "query": {"match_all": {}},
                "random_score": {},
                "boost_mode": "replace"
            }
        },
        "size": 1
    }
    
    try:
        response = es.search(index=index_name, body=query)
        
        total = response['hits']['total']['value']
        if total == 0:
            print("✗ No documents found in index")
            return None
        
        if len(response['hits']['hits']) == 0:
            print("✗ No documents returned")
            return None
        
        # Return the document source
        return response['hits']['hits'][0]['_source']
        
    except (TransportError, RequestError) as e:
        print(f"✗ Error querying Elasticsearch: {e}")
        return None


def main():
    """Main function."""
    # Load config and connect
    config = load_config()
    es = connect_elasticsearch(config)
    index_name = config['elasticsearch']['index_name']
    
    print(f"Fetching random document from index: {index_name}...", file=sys.stderr)
    
    # Get random document
    doc = get_random_document(es, index_name)
    
    if doc:
        # Output as JSON
        print(json.dumps(doc, ensure_ascii=False, indent=2))
    else:
        sys.exit(1)


if __name__ == "__main__":
    main()
