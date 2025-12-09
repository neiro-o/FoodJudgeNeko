#!/usr/bin/env python3
"""
Reset Elasticsearch Index

This script resets the Elasticsearch index by:
1. Deleting the existing index (if it exists)
2. Recreating it with proper mappings and settings

Usage:
    python reset_es_index.py          # Reset index (delete + recreate)
    python reset_es_index.py --delete # Only delete index
    python reset_es_index.py --create # Only create index (skip if exists)
"""

import sys
import argparse
import yaml
from pathlib import Path

# Import from es_init
from es_init import init_elasticsearch, create_index, get_es_mapping
from elasticsearch import Elasticsearch
from elasticsearch.exceptions import TransportError, RequestError, NotFoundError


def load_config(config_path: str = "../config.yml") -> dict:
    """Load configuration from YAML file."""
    try:
        config_file = Path(__file__).parent / config_path
        with open(config_file, 'r', encoding='utf-8') as f:
            config = yaml.safe_load(f)
        return config
    except Exception as e:
        print(f"✗ Error loading config: {e}")
        sys.exit(1)


def connect_elasticsearch(config: dict) -> Elasticsearch:
    """Connect to Elasticsearch."""
    es_config = config.get('elasticsearch', {})
    hosts = es_config.get('hosts', ['http://localhost:9200'])
    username = es_config.get('username', '')
    password = es_config.get('password', '')
    
    es_config_dict = {
        'hosts': hosts,
        'request_timeout': 30
    }
    
    if username and password:
        es_config_dict['basic_auth'] = (username, password)
    
    # Check if HTTPS
    use_https = any(host.startswith('https://') for host in hosts)
    if use_https:
        es_config_dict['verify_certs'] = False
        es_config_dict['ssl_show_warn'] = False
    
    es = Elasticsearch(**es_config_dict)
    
    if not es.ping():
        print("✗ Cannot connect to Elasticsearch")
        sys.exit(1)
    
    return es


def delete_index(es: Elasticsearch, index_name: str) -> bool:
    """Delete Elasticsearch index."""
    try:
        if es.indices.exists(index=index_name):
            # Get document count before deletion
            try:
                count = es.count(index=index_name)['count']
                print(f"⚠️  Index '{index_name}' contains {count:,} documents")
            except:
                pass
            
            es.indices.delete(index=index_name)
            print(f"✓ Deleted index '{index_name}'")
            return True
        else:
            print(f"ℹ️  Index '{index_name}' does not exist (nothing to delete)")
            return False
    except NotFoundError:
        print(f"ℹ️  Index '{index_name}' does not exist (nothing to delete)")
        return False
    except (TransportError, RequestError) as e:
        print(f"✗ Failed to delete index: {e}")
        return False


def create_index_safe(es: Elasticsearch, index_name: str) -> bool:
    """Create Elasticsearch index with proper mapping."""
    try:
        if es.indices.exists(index=index_name):
            print(f"⚠️  Index '{index_name}' already exists")
            return False
        
        mapping = get_es_mapping()
        es.indices.create(index=index_name, body=mapping)
        print(f"✓ Created index '{index_name}' with proper mappings")
        return True
    except RequestError as e:
        if "resource_already_exists_exception" in str(e):
            print(f"⚠️  Index '{index_name}' already exists")
            return False
        else:
            print(f"✗ Failed to create index: {e}")
            return False
    except (TransportError, RequestError) as e:
        print(f"✗ Failed to create index: {e}")
        return False


def reset_index(es: Elasticsearch, index_name: str) -> bool:
    """Reset index: delete and recreate."""
    print("\n" + "="*80)
    print("RESETTING ELASTICSEARCH INDEX")
    print("="*80 + "\n")
    
    print(f"Index: {index_name}\n")
    
    # Step 1: Delete
    print("[1/2] Deleting existing index...")
    deleted = delete_index(es, index_name)
    
    # Step 2: Create
    print("\n[2/2] Creating new index...")
    created = create_index_safe(es, index_name)
    
    if created:
        print("\n" + "="*80)
        print("✅ Index reset complete!")
        print("="*80)
        print("\nNext steps:")
        print("  1. Run sync script to populate index:")
        print("     cd ../ESsync && python sync_to_es.py")
        print("  2. Verify document count:")
        print("     python count_docs.py")
        print()
        return True
    else:
        print("\n⚠️  Index reset may not have completed successfully")
        return False


def main():
    parser = argparse.ArgumentParser(
        description="Reset Elasticsearch index",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python reset_es_index.py              # Reset (delete + recreate)
  python reset_es_index.py --delete     # Only delete
  python reset_es_index.py --create     # Only create
        """
    )
    parser.add_argument(
        '--delete',
        action='store_true',
        help='Only delete the index (do not recreate)'
    )
    parser.add_argument(
        '--create',
        action='store_true',
        help='Only create the index (do not delete)'
    )
    parser.add_argument(
        '--config',
        type=str,
        default='../config.yml',
        help='Path to config.yml (default: ../config.yml)'
    )
    
    args = parser.parse_args()
    
    # Load config
    config = load_config(args.config)
    index_name = config.get('elasticsearch', {}).get('index_name', 'problems')
    
    # Connect to ES
    print("Connecting to Elasticsearch...")
    es = connect_elasticsearch(config)
    print("✓ Connected to Elasticsearch\n")
    
    # Execute based on flags
    if args.delete:
        # Only delete
        print("="*80)
        print("DELETING ELASTICSEARCH INDEX")
        print("="*80 + "\n")
        delete_index(es, index_name)
    elif args.create:
        # Only create
        print("="*80)
        print("CREATING ELASTICSEARCH INDEX")
        print("="*80 + "\n")
        create_index_safe(es, index_name)
    else:
        # Reset (delete + create)
        reset_index(es, index_name)


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\n\n⚠️  Interrupted by user")
        sys.exit(1)
    except Exception as e:
        print(f"\n✗ Unexpected error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
