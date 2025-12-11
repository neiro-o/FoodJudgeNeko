#!/usr/bin/env python3
"""
Count documents in Elasticsearch index

Quick script to check how many documents are in the problems index.
"""

import sys
import yaml
from pathlib import Path

try:
    from elasticsearch import Elasticsearch
    from elasticsearch.exceptions import TransportError, RequestError
except ImportError:
    print("Error: elasticsearch not installed. Run: pip install elasticsearch")
    sys.exit(1)


def main():
    # Load config
    config_file = Path(__file__).parent / "../config.yml"
    try:
        with open(config_file, 'r', encoding='utf-8') as f:
            config = yaml.safe_load(f)
    except Exception as e:
        print(f"Error loading config: {e}")
        sys.exit(1)
    
    # Connect to ES
    es_config = config['elasticsearch']
    try:
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
        
        if not es.ping():
            print("✗ Cannot connect to Elasticsearch")
            sys.exit(1)
    except Exception as e:
        print(f"✗ Elasticsearch connection failed: {e}")
        sys.exit(1)
    
    index_name = es_config['index_name']
    
    print("\n" + "="*80)
    print("Elasticsearch Index Statistics")
    print("="*80 + "\n")
    
    try:
        # Get document count
        count_response = es.count(index=index_name)
        total_docs = count_response['count']
        
        print(f"Index: {index_name}")
        print(f"Total Documents: {total_docs:,}")
        
        # Get index stats for more details
        try:
            stats = es.indices.stats(index=index_name)
            if index_name in stats.get('indices', {}):
                index_stats = stats['indices'][index_name]
                print(f"\nIndex Size: {index_stats['total']['store']['size_in_bytes']:,} bytes")
                if 'primaries' in index_stats:
                    print(f"Primary Shards: {index_stats['primaries'].get('count', 'N/A')}")
        except Exception as e:
            print(f"\n(Stats unavailable: {e})")
        
        # Count by problem type
        print("\n" + "-"*80)
        print("Documents by Problem Type:")
        print("-"*80)
        
        for ptype in [1, 2, 3, 4]:
            query = {
                "query": {
                    "term": {
                        "problem_type": ptype
                    }
                }
            }
            count = es.count(index=index_name, body=query)['count']
            print(f"  Type {ptype}: {count:,} documents")
        
        # Check for Type 5 (should be 0)
        query = {
            "query": {
                "term": {
                    "problem_type": 5
                }
            }
        }
        count = es.count(index=index_name, body=query)['count']
        if count > 0:
            print(f"  Type 5: {count:,} documents (⚠️  unexpected)")
        
        print("\n" + "="*80)
        
    except (TransportError, RequestError) as e:
        print(f"✗ Error querying Elasticsearch: {e}")
        sys.exit(1)
    except KeyError as e:
        print(f"✗ Index '{index_name}' may not exist: {e}")
        print("\nAvailable indices:")
        try:
            indices = es.indices.get_alias()
            for idx in indices.keys():
                print(f"  - {idx}")
        except:
            pass
        sys.exit(1)


if __name__ == "__main__":
    main()
