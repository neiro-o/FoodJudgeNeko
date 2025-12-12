#!/usr/bin/env python3
"""
Elasticsearch Search Example Script

This script demonstrates how to search the problems index in Elasticsearch.
Includes examples for keyword search, field-specific search, and advanced queries.
"""

import sys
import re
import yaml
from pathlib import Path
from typing import Dict, List, Any

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
            print("✓ Connected to Elasticsearch")
            return es
        else:
            print("✗ Elasticsearch ping failed")
            sys.exit(1)
    except Exception as e:
        print(f"✗ Elasticsearch connection failed: {e}")
        sys.exit(1)


def search_keyword(es: Elasticsearch, index_name: str, keyword: str, size: int = 10, min_score: float = 1.0):
    """
    Search for keyword across multiple fields.
    
    Args:
        es: Elasticsearch client
        index_name: Index name
        keyword: Search keyword
        size: Number of results to return
        min_score: Minimum score threshold (default: 1.0)
    """
    print(f"\n{'='*80}")
    print(f"Searching for keyword: '{keyword}'")
    print(f"Minimum score: {min_score:.2f}")
    print(f"{'='*80}\n")
    
    # Multi-match query - searches across multiple fields
    query = {
        "query": {
            "multi_match": {
                "query": keyword,
                "fields": [
                    "user_review^3",      # Boost user_review (3x weight)
                    "content^2",           # Boost content in replies/appeals (2x weight)
                    "appeals.content^2",
                    "replies.content^2",
                    "comments.content",
                    "orders.name",
                    "orders.desc"
                ],
                "type": "best_fields",    # Use best matching field
                "fuzziness": "AUTO"       # Allow typos
            }
        },
        "min_score": min_score,  # Filter out low-scoring results
        "size": size,
        "highlight": {
            "fields": {
                "user_review": {},
                "content": {},
                "appeals.content": {},
                "replies.content": {},
                "comments.content": {}
            }
        }
    }
    
    try:
        response = es.search(index=index_name, body=query)
        
        total = response['hits']['total']['value']
        returned = len(response['hits']['hits'])
        print(f"Found {total} document(s) (showing {returned} above score {min_score:.2f})\n")
        
        if total > 0:
            for i, hit in enumerate(response['hits']['hits'], 1):
                doc = hit['_source']
                score = hit['_score']
                
                print(f"[{i}] Score: {score:.2f}")
                print(f"    mongo_id: {doc.get('mongo_id')}")
                print(f"    problem_type: {doc.get('problem_type')}")
                print(f"    user_review: {doc.get('user_review', '')[:100]}...")
                
                # Show highlights if available
                if 'highlight' in hit:
                    print("    Highlights:")
                    for field, highlights in hit['highlight'].items():
                        for highlight in highlights:
                            print(f"      {field}: ...{highlight}...")
                
                print()
        else:
            print("No documents found.")
            
        return response
        
    except (TransportError, RequestError) as e:
        print(f"✗ Search error: {e}")
        return None


def search_in_field(es: Elasticsearch, index_name: str, keyword: str, field: str, size: int = 10):
    """
    Search for keyword in a specific field.
    
    Args:
        es: Elasticsearch client
        index_name: Index name
        keyword: Search keyword
        field: Field to search in
        size: Number of results to return
    """
    print(f"\n{'='*80}")
    print(f"Searching for '{keyword}' in field: {field}")
    print(f"{'='*80}\n")
    
    query = {
        "query": {
            "match": {
                field: {
                    "query": keyword,
                    "fuzziness": "AUTO"
                }
            }
        },
        "size": size,
        "highlight": {
            "fields": {
                field: {}
            }
        }
    }
    
    try:
        response = es.search(index=index_name, body=query)
        
        total = response['hits']['total']['value']
        print(f"Found {total} document(s)\n")
        
        if total > 0:
            for i, hit in enumerate(response['hits']['hits'], 1):
                doc = hit['_source']
                print(f"[{i}] mongo_id: {doc.get('mongo_id')}")
                print(f"    {field}: {doc.get(field, '')[:150]}...")
                
                if 'highlight' in hit and field in hit['highlight']:
                    print(f"    Highlight: ...{hit['highlight'][field][0]}...")
                print()
        else:
            print("No documents found.")
            
        return response
        
    except (TransportError, RequestError) as e:
        print(f"✗ Search error: {e}")
        return None


def search_by_type(es: Elasticsearch, index_name: str, problem_type: int, size: int = 10):
    """Search documents by problem type."""
    print(f"\n{'='*80}")
    print(f"Searching for problem_type: {problem_type}")
    print(f"{'='*80}\n")
    
    query = {
        "query": {
            "term": {
                "problem_type": problem_type
            }
        },
        "size": size
    }
    
    try:
        response = es.search(index=index_name, body=query)
        total = response['hits']['total']['value']
        print(f"Found {total} document(s) of type {problem_type}\n")
        return response
    except (TransportError, RequestError) as e:
        print(f"✗ Search error: {e}")
        return None


def search_user_review_contains(es: Elasticsearch, index_name: str, search_string: str, size: int = 10):
    """
    Search for documents where user_review field contains the specified string.
    Uses simple string inclusion (substring matching), not indexed full-text search.
    
    Args:
        es: Elasticsearch client
        index_name: Index name
        search_string: String to search for in user_review field
        size: Number of results to return
    """
    print(f"\n{'='*80}")
    print(f"Searching for documents where user_review contains: '{search_string}'")
    print(f"(Simple string inclusion, no indexing)")
    print(f"{'='*80}\n")
    
    # Escape special characters for wildcard query
    # Escape wildcard special characters: *, ?, \
    escaped_string = re.sub(r'([*?\\])', r'\\\1', search_string)
    
    query = {
        "query": {
            "wildcard": {
                "user_review": {
                    "value": f"*{escaped_string}*",
                    "case_insensitive": True
                }
            }
        },
        "size": size
    }
    
    try:
        response = es.search(index=index_name, body=query)
        
        total = response['hits']['total']['value']
        print(f"Found {total} document(s)\n")
        
        if total > 0:
            for i, hit in enumerate(response['hits']['hits'], 1):
                doc = hit['_source']
                user_review = doc.get('user_review', '')
                
                print(f"[{i}] mongo_id: {doc.get('mongo_id')}")
                print(f"    problem_type: {doc.get('problem_type')}")
                print(f"    user_review: {user_review[:200]}...")
                
                # Show where the string was found
                if search_string.lower() in user_review.lower():
                    idx = user_review.lower().find(search_string.lower())
                    start = max(0, idx - 30)
                    end = min(len(user_review), idx + len(search_string) + 30)
                    context = user_review[start:end]
                    print(f"    Match context: ...{context}...")
                print()
        else:
            print("No documents found.")
            
        return response
        
    except (TransportError, RequestError) as e:
        print(f"✗ Search error: {e}")
        return None


def search_combined(es: Elasticsearch, index_name: str, keyword: str, problem_type: int = None, size: int = 10):
    """
    Combined search: keyword + optional problem type filter.
    """
    print(f"\n{'='*80}")
    print(f"Combined search: keyword='{keyword}'" + (f", type={problem_type}" if problem_type else ""))
    print(f"{'='*80}\n")
    
    must_clauses = [
        {
            "multi_match": {
                "query": keyword,
                "fields": ["user_review^3", "content^2", "appeals.content^2", "replies.content^2"],
                "fuzziness": "AUTO"
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
        "size": size,
        "highlight": {
            "fields": {
                "user_review": {},
                "content": {}
            }
        }
    }
    
    try:
        response = es.search(index=index_name, body=query)
        total = response['hits']['total']['value']
        print(f"Found {total} document(s)\n")
        
        if total > 0:
            for i, hit in enumerate(response['hits']['hits'], 1):
                doc = hit['_source']
                print(f"[{i}] Type {doc.get('problem_type')} - {doc.get('mongo_id')}")
                print(f"    Review: {doc.get('user_review', '')[:100]}...")
                print()
        
        return response
    except (TransportError, RequestError) as e:
        print(f"✗ Search error: {e}")
        return None


def main():
    """Main function with example searches."""
    print("""
╔════════════════════════════════════════════════════════════════════════════╗
║                    Elasticsearch Search Examples                             ║
╚════════════════════════════════════════════════════════════════════════════╝
    """)
    
    # Load config and connect
    config = load_config()
    es = connect_elasticsearch(config)
    index_name = config['elasticsearch']['index_name']
    
    # Example 1: Search for specific keyword across all fields
    keyword = "和图片bu符"
    print(f"\n{'='*80}")
    print("EXAMPLE 1: Multi-field keyword search")
    print(f"{'='*80}")
    search_keyword(es, index_name, keyword, size=5)
    
    # Example 2: Search in specific field (user_review)
    print(f"\n{'='*80}")
    print("EXAMPLE 2: Search in user_review field only")
    print(f"{'='*80}")
    search_in_field(es, index_name, keyword, "user_review", size=5)
    
    # Example 3: Combined search (keyword + type filter)
    print(f"\n{'='*80}")
    print("EXAMPLE 3: Combined search (keyword + type filter)")
    print(f"{'='*80}")
    search_combined(es, index_name, keyword, problem_type=1, size=5)
    
    # Example 4: Search by type only
    print(f"\n{'='*80}")
    print("EXAMPLE 4: Search by problem type")
    print(f"{'='*80}")
    search_by_type(es, index_name, problem_type=1, size=5)
    
    # Example 5: Simple string inclusion search in user_review
    search_str = "份量很足"
    print(f"\n{'='*80}")
    print("EXAMPLE 5: Simple string inclusion search in user_review")
    print(f"{'='*80}")
    search_user_review_contains(es, index_name, search_str, size=5)
    
    print("\n" + "="*80)
    print("Search examples completed!")
    print("="*80)


if __name__ == "__main__":
    main()
