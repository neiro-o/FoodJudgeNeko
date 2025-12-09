#!/usr/bin/env python3
"""
Elasticsearch search examples for Chinese keyword search using IK Analyzer.
"""

from elasticsearch import Elasticsearch
from typing import Dict, Any


def search_by_keywords(es: Elasticsearch, index_name: str, keywords: str,
                      size: int = 10, from_: int = 0) -> Dict[str, Any]:
    """
    Search keywords in user_review, replies.content, and appeals.content.
    
    Args:
        es: Elasticsearch client
        index_name: Index name
        keywords: Search keywords (Chinese supported)
        size: Number of results
        from_: Pagination offset
    
    Returns:
        Search results
    """
    query = {
        "query": {
            "multi_match": {
                "query": keywords,
                "fields": ["searchable_content^2", "user_review^1.5"],
                "type": "best_fields",
                "operator": "or",
                "fuzziness": "AUTO"
            }
        },
        "size": size,
        "from": from_,
        "highlight": {
            "fields": {
                "searchable_content": {"number_of_fragments": 3, "fragment_size": 150},
                "user_review": {"number_of_fragments": 2, "fragment_size": 150}
            },
            "pre_tags": ["<mark>"],
            "post_tags": ["</mark>"]
        }
    }
    return es.search(index=index_name, body=query)


def search_nested(es: Elasticsearch, index_name: str, keywords: str,
                 size: int = 10, from_: int = 0) -> Dict[str, Any]:
    """
    Search with nested queries for replies and appeals.
    
    Args:
        es: Elasticsearch client
        index_name: Index name
        keywords: Search keywords
        size: Number of results
        from_: Pagination offset
    
    Returns:
        Search results
    """
    query = {
        "query": {
            "bool": {
                "should": [
                    {"match": {"user_review": {"query": keywords, "boost": 2.0}}},
                    {
                        "nested": {
                            "path": "replies",
                            "query": {"match": {"replies.content": {"query": keywords, "boost": 1.5}}},
                            "inner_hits": {"highlight": {"fields": {"replies.content": {}}}}
                        }
                    },
                    {
                        "nested": {
                            "path": "appeals",
                            "query": {"match": {"appeals.content": {"query": keywords, "boost": 1.5}}},
                            "inner_hits": {"highlight": {"fields": {"appeals.content": {}}}}
                        }
                    }
                ],
                "minimum_should_match": 1
            }
        },
        "size": size,
        "from": from_,
        "highlight": {
            "fields": {"user_review": {}},
            "pre_tags": ["<mark>"],
            "post_tags": ["</mark>"]
        }
    }
    return es.search(index=index_name, body=query)


def search_with_filters(es: Elasticsearch, index_name: str, keywords: str,
                       problem_type: int = None, answer: int = None,
                       size: int = 10, from_: int = 0) -> Dict[str, Any]:
    """
    Search with filters (problem_type, answer).
    
    Args:
        es: Elasticsearch client
        index_name: Index name
        keywords: Search keywords
        problem_type: Filter by problem type (optional)
        answer: Filter by answer (optional)
        size: Number of results
        from_: Pagination offset
    
    Returns:
        Search results
    """
    must_clauses = [{"multi_match": {"query": keywords, "fields": ["searchable_content^2", "user_review^1.5"]}}]
    
    if problem_type is not None:
        must_clauses.append({"term": {"problem_type": problem_type}})
    if answer is not None:
        must_clauses.append({"term": {"answer": answer}})
    
    query = {
        "query": {"bool": {"must": must_clauses}},
        "size": size,
        "from": from_,
        "highlight": {
            "fields": {
                "searchable_content": {"number_of_fragments": 3},
                "user_review": {"number_of_fragments": 2}
            }
        }
    }
    return es.search(index=index_name, body=query)


# Example usage
if __name__ == "__main__":
    from pathlib import Path
    import yaml
    
    config_path = Path(__file__).parent.parent / "config.yml"
    with open(config_path, 'r') as f:
        config = yaml.safe_load(f)
    
    hosts = config.get('elasticsearch', {}).get('hosts', ['http://localhost:9200'])
    index_name = config.get('elasticsearch', {}).get('index_name', 'problems')
    username = config.get('elasticsearch', {}).get('username', '')
    password = config.get('elasticsearch', {}).get('password', '')
    
    es_config = {'hosts': hosts}
    if username and password:
        es_config['basic_auth'] = (username, password)
    
    # Disable SSL verification for HTTPS connections (development only)
    use_https = any(host.startswith('https://') for host in hosts)
    if use_https:
        es_config['verify_certs'] = False
        es_config['ssl_show_warn'] = False
    
    es = Elasticsearch(**es_config)
    
    # Examples
    results = search_by_keywords(es, index_name, "退款问题")
    print(f"Found {results['hits']['total']['value']} results")
    
    results = search_nested(es, index_name, "商家回复")
    print(f"Found {results['hits']['total']['value']} results")
    
    results = search_with_filters(es, index_name, "退款", problem_type=3)
    print(f"Found {results['hits']['total']['value']} results")
