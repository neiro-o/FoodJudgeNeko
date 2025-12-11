#!/usr/bin/env python3
"""
Elasticsearch Diagnostic Script

This script helps diagnose issues with problem_type != 1 documents.
It checks:
1. Document counts by problem_type
2. Sample documents of each type
3. Search functionality for each type
4. Index mapping for problem_type field

Usage:
    python diagnose_es.py              # Full diagnostic
    python diagnose_es.py --type 2     # Check specific type
    python diagnose_es.py --search     # Test search for each type
"""

import sys
import yaml
import json
import argparse
from pathlib import Path
from typing import Dict, Any, List
from collections import defaultdict

try:
    from elasticsearch import Elasticsearch
    from elasticsearch.exceptions import TransportError, RequestError, NotFoundError
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
        print(f"✗ Error loading config: {e}")
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
            print("✓ Connected to Elasticsearch")
            return es
        else:
            print("✗ Elasticsearch ping failed")
            sys.exit(1)
    except Exception as e:
        print(f"✗ Elasticsearch connection failed: {e}")
        sys.exit(1)


def get_document_counts_by_type(es: Elasticsearch, index_name: str) -> Dict[int, int]:
    """Get document counts grouped by problem_type."""
    print("\n" + "=" * 80)
    print("DOCUMENT COUNTS BY PROBLEM TYPE")
    print("=" * 80)
    
    counts = {}
    
    # Check if index exists
    if not es.indices.exists(index=index_name):
        print(f"✗ Index '{index_name}' does not exist")
        return counts
    
    # Get total count
    try:
        total = es.count(index=index_name)['count']
        print(f"Total documents: {total:,}")
    except Exception as e:
        print(f"✗ Failed to get total count: {e}")
        return counts
    
    # Get counts by type using aggregation
    try:
        query = {
            "size": 0,
            "aggs": {
                "by_type": {
                    "terms": {
                        "field": "problem_type",
                        "size": 10
                    }
                }
            }
        }
        response = es.search(index=index_name, body=query)
        
        buckets = response['aggregations']['by_type']['buckets']
        print("\nBreakdown by problem_type:")
        for bucket in buckets:
            ptype = bucket['key']
            count = bucket['doc_count']
            counts[ptype] = count
            print(f"  Type {ptype}: {count:,} document(s)")
        
        # Check for missing types
        all_types = [1, 2, 3, 4]
        missing = [t for t in all_types if t not in counts]
        if missing:
            print(f"\n⚠️  Missing types: {missing}")
        
    except Exception as e:
        print(f"✗ Failed to get counts by type: {e}")
        # Fallback: try to get a few documents and check manually
        try:
            response = es.search(index=index_name, body={"size": 100})
            type_counts = defaultdict(int)
            for hit in response['hits']['hits']:
                ptype = hit['_source'].get('problem_type')
                if ptype:
                    type_counts[ptype] += 1
            
            print("\nBreakdown (from sample):")
            for ptype, count in sorted(type_counts.items()):
                print(f"  Type {ptype}: {count} document(s) in sample")
                counts[ptype] = count
        except Exception as e2:
            print(f"✗ Fallback also failed: {e2}")
    
    return counts


def get_sample_documents(es: Elasticsearch, index_name: str, problem_type: int, size: int = 3) -> List[Dict]:
    """Get sample documents of a specific problem_type."""
    try:
        query = {
            "query": {
                "term": {
                    "problem_type": problem_type
                }
            },
            "size": size
        }
        response = es.search(index=index_name, body=query)
        return [hit['_source'] for hit in response['hits']['hits']]
    except Exception as e:
        print(f"✗ Failed to get sample documents for type {problem_type}: {e}")
        return []


def check_index_mapping(es: Elasticsearch, index_name: str):
    """Check the index mapping, especially for problem_type field."""
    print("\n" + "=" * 80)
    print("INDEX MAPPING CHECK")
    print("=" * 80)
    
    if not es.indices.exists(index=index_name):
        print(f"✗ Index '{index_name}' does not exist")
        return
    
    try:
        mapping = es.indices.get_mapping(index=index_name)
        index_mapping = mapping[index_name]['mappings']
        
        # Check if problem_type field exists
        properties = index_mapping.get('properties', {})
        if 'problem_type' in properties:
            pt_mapping = properties['problem_type']
            print(f"✓ problem_type field exists")
            print(f"  Type: {pt_mapping.get('type', 'unknown')}")
            if 'fields' in pt_mapping:
                print(f"  Additional fields: {list(pt_mapping['fields'].keys())}")
        else:
            print("✗ problem_type field NOT found in mapping!")
            print("  This could cause issues with searching/filtering by problem_type")
        
        # Show all field types
        print(f"\nTotal fields in mapping: {len(properties)}")
        print("\nKey fields:")
        key_fields = ['problem_type', 'mongo_id', 'user_review', 'timestamp', 'answer']
        for field in key_fields:
            if field in properties:
                field_type = properties[field].get('type', 'unknown')
                print(f"  {field}: {field_type}")
        
    except Exception as e:
        print(f"✗ Failed to get mapping: {e}")


def test_search_by_type(es: Elasticsearch, index_name: str, problem_type: int):
    """Test searching for a specific problem_type."""
    print(f"\n{'=' * 80}")
    print(f"SEARCH TEST FOR TYPE {problem_type}")
    print("=" * 80)
    
    # Test 1: Term query
    print("\n[Test 1] Term query (exact match):")
    try:
        query = {
            "query": {
                "term": {
                    "problem_type": problem_type
                }
            },
            "size": 5
        }
        response = es.search(index=index_name, body=query)
        total = response['hits']['total']['value']
        returned = len(response['hits']['hits'])
        print(f"  Found: {total} document(s), showing {returned}")
        
        if returned > 0:
            for i, hit in enumerate(response['hits']['hits'][:3], 1):
                doc = hit['_source']
                print(f"  [{i}] mongo_id: {doc.get('mongo_id')}, type: {doc.get('problem_type')}")
        else:
            print(f"  ⚠️  No documents found for type {problem_type}")
    except Exception as e:
        print(f"  ✗ Term query failed: {e}")
    
    # Test 2: Match query (in case problem_type is text)
    print("\n[Test 2] Match query:")
    try:
        query = {
            "query": {
                "match": {
                    "problem_type": str(problem_type)
                }
            },
            "size": 5
        }
        response = es.search(index=index_name, body=query)
        total = response['hits']['total']['value']
        print(f"  Found: {total} document(s)")
    except Exception as e:
        print(f"  ✗ Match query failed: {e}")
    
    # Test 3: Get a sample document to verify structure
    print("\n[Test 3] Sample document structure:")
    samples = get_sample_documents(es, index_name, problem_type, size=1)
    if samples:
        sample = samples[0]
        print(f"  ✓ Found sample document")
        print(f"    mongo_id: {sample.get('mongo_id')}")
        print(f"    problem_type: {sample.get('problem_type')}")
        print(f"    Has user_review: {'user_review' in sample}")
        print(f"    Has timestamp: {'timestamp' in sample}")
    else:
        print(f"  ✗ No sample documents found for type {problem_type}")


def test_keyword_search(es: Elasticsearch, index_name: str, problem_type: int = None):
    """Test keyword search with optional type filter."""
    print(f"\n{'=' * 80}")
    print(f"KEYWORD SEARCH TEST" + (f" (filtered by type {problem_type})" if problem_type else ""))
    print("=" * 80)
    
    keyword = "test"  # Generic keyword
    
    must_clauses = [
        {
            "match": {
                "user_review": {
                    "query": keyword,
                    "operator": "or"
                }
            }
        }
    ]
    
    if problem_type:
        must_clauses.append({
            "term": {"problem_type": problem_type}
        })
    
    query = {
        "query": {
            "bool": {
                "must": must_clauses
            }
        },
        "size": 5
    }
    
    try:
        response = es.search(index=index_name, body=query)
        total = response['hits']['total']['value']
        returned = len(response['hits']['hits'])
        print(f"Found: {total} document(s), showing {returned}")
        
        if returned > 0:
            type_counts = defaultdict(int)
            for hit in response['hits']['hits']:
                ptype = hit['_source'].get('problem_type')
                type_counts[ptype] += 1
            
            print("\nResults by type:")
            for ptype, count in sorted(type_counts.items()):
                print(f"  Type {ptype}: {count} document(s)")
    except Exception as e:
        print(f"✗ Search failed: {e}")


def main():
    parser = argparse.ArgumentParser(
        description="Diagnose Elasticsearch index for problem_type issues",
        formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument(
        '--type',
        type=int,
        help='Check specific problem_type (1-4)'
    )
    parser.add_argument(
        '--search',
        action='store_true',
        help='Test search functionality for each type'
    )
    parser.add_argument(
        '--config',
        type=str,
        default='../config.yml',
        help='Path to config.yml (default: ../config.yml)'
    )
    
    args = parser.parse_args()
    
    print("""
╔════════════════════════════════════════════════════════════════════════════╗
║              Elasticsearch Diagnostic Tool                                 ║
║              For problem_type != 1 Issues                                 ║
╚════════════════════════════════════════════════════════════════════════════╝
    """)
    
    # Load config and connect
    config = load_config(args.config)
    es = connect_elasticsearch(config)
    index_name = config['elasticsearch']['index_name']
    
    # Check mapping
    check_index_mapping(es, index_name)
    
    # Get document counts
    counts = get_document_counts_by_type(es, index_name)
    
    # If specific type requested, focus on that
    if args.type:
        if args.type not in counts:
            print(f"\n⚠️  No documents found for type {args.type}")
        else:
            print(f"\n✓ Found {counts[args.type]:,} document(s) of type {args.type}")
            test_search_by_type(es, index_name, args.type)
            if args.search:
                test_keyword_search(es, index_name, args.type)
    else:
        # Check all types
        for ptype in [1, 2, 3, 4]:
            if ptype in counts and counts[ptype] > 0:
                print(f"\n{'=' * 80}")
                print(f"CHECKING TYPE {ptype} ({counts[ptype]:,} documents)")
                print("=" * 80)
                test_search_by_type(es, index_name, ptype)
                if args.search:
                    test_keyword_search(es, index_name, ptype)
            else:
                print(f"\n⚠️  Type {ptype}: No documents found")
    
    # Summary
    print("\n" + "=" * 80)
    print("SUMMARY")
    print("=" * 80)
    print("\nDocument counts:")
    for ptype in [1, 2, 3, 4]:
        count = counts.get(ptype, 0)
        status = "✓" if count > 0 else "✗"
        print(f"  {status} Type {ptype}: {count:,} document(s)")
    
    print("\n" + "=" * 80)


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
