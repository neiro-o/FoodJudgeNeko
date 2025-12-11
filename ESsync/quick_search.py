#!/usr/bin/env python3
"""
Quick Elasticsearch Search

Simple script to search for a keyword in the problems index.
Searches across all searchable fields from ES_Params.md with improved fuzziness.

Features:
- Dynamic fuzziness (scales with keyword length: 0-4 chars difference)
- Searches all content fields: user_review, replies, appeals, comments, orders
- Nested field support for arrays
- Maximum score (not sum) - matching once is enough, multiple matches don't boost score
- Fuzzy match score boosting - fuzzy matches get boosted to maintain high scores
- Field boosting (user_review has highest priority)
- Minimum score threshold to filter low-relevance results

Fuzziness by keyword length:
- 1-4 chars: fuzziness = 0
- 5-8 chars: fuzziness = 1
- 9-15 chars: fuzziness = 2
- 16+ chars: fuzziness = 4

Usage: 
    python quick_search.py "和图片bu符"                    # Default min_score: 1.0, all problem types
    python quick_search.py "和图片bu符" 2.0                # Custom min_score: 2.0
    python quick_search.py "和图片bu符" 1.0 4              # Filter by problem_type == 4
    python quick_search.py "和图片bu符" 2.0 4             # min_score: 2.0, problem_type: 4
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


def calculate_fuzziness(keyword: str) -> int:
    """
    Calculate fuzziness based on keyword length.
    
    Formula: fuzziness scales with keyword length
    - Very short (1-4 chars): fuzziness = 0
    - Short-medium (5-8 chars): fuzziness = 1
    - Medium-long (9-15 chars): fuzziness = 2
    - Very long (16+ chars): fuzziness = 4
    
    Args:
        keyword: Search keyword
        
    Returns:
        Integer fuzziness value (1-4)
    """
    length = len(keyword)
    
    # Fuzziness scales with keyword length
    # Shorter keywords need less fuzziness (more precise)
    # Longer keywords can tolerate more fuzziness (more forgiving)
    if length <= 4:
        return 0  # Very short: allow 0 char difference
    elif length <= 8:
        return 1  # Short-medium: allow 1 char difference
    elif length <= 15:
        return 2  # Medium-long: allow 2 char differences
    else:
        return 4  # Very long: allow 4 char differences (capped)


def main():
    # Get keyword, min_score, and problem_type from command line
    keyword = sys.argv[1] if len(sys.argv) > 1 else "和图片bu符"
    
    # Get minimum score threshold (default: 1.0)
    # Usage: python quick_search.py "keyword" 2.0
    min_score = float(sys.argv[2]) if len(sys.argv) > 2 else 1.0
    
    # Get problem_type filter (default: None = all types)
    # Usage: python quick_search.py "keyword" 1.0 4
    problem_type_filter = int(sys.argv[3]) if len(sys.argv) > 3 else None
    
    # Calculate dynamic fuzziness based on keyword length
    fuzziness = calculate_fuzziness(keyword)
    
    # Load config
    config_file = Path(__file__).parent / "../config.yml"
    with open(config_file, 'r', encoding='utf-8') as f:
        config = yaml.safe_load(f)
    
    # Connect to ES
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
    
    index_name = es_config['index_name']
    
    # Search query - includes all searchable fields from ES_Params.md
    # Use dis_max query to get maximum score (not sum) - matching once is enough
    # Optionally filter by problem_type
    bool_query = {
        "must": [
            {
                "dis_max": {
                    "queries": [
                    # Top-level text fields - boost fuzzy matches to maintain score
                    {
                        "function_score": {
                            "query": {
                                "match": {
                                    "user_review": {
                                        "query": keyword,
                                        "fuzziness": fuzziness,
                                        "prefix_length": 1,
                                        "operator": "and"
                                    }
                                }
                            },
                            "script_score": {
                                "script": {
                                    "source": "_score * 1.2 + 0.5",  # Boost fuzzy matches: multiply by 1.2 and add 0.5
                                    "lang": "painless"
                                }
                            },
                            "boost": 1.0,  # Field boost
                            "boost_mode": "multiply"
                        }
                    },
                    {
                        "function_score": {
                            "query": {
                                "match": {
                                    "others": {
                                        "query": keyword,
                                        "fuzziness": fuzziness,
                                        "prefix_length": 1
                                    }
                                }
                            },
                            "script_score": {
                                "script": {
                                    "source": "_score * 1.2 + 0.5",  # Boost fuzzy matches
                                    "lang": "painless"
                                }
                            },
                            "boost": 0.15,
                            "boost_mode": "multiply"
                        }
                    },
                    # Nested: replies.content
                    {
                        "nested": {
                            "path": "replies",
                            "query": {
                                "function_score": {
                                    "query": {
                                        "match": {
                                            "replies.content": {
                                                "query": keyword,
                                                "fuzziness": fuzziness,
                                                "prefix_length": 1
                                            }
                                        }
                                    },
                                    "script_score": {
                                        "script": {
                                            "source": "_score * 1.2 + 0.5",  # Boost fuzzy matches
                                            "lang": "painless"
                                        }
                                    },
                                    "boost": 1.0,
                                    "boost_mode": "multiply"
                                }
                            }
                        }
                    },
                    # Nested: appeals.content
                    {
                        "nested": {
                            "path": "appeals",
                            "query": {
                                "function_score": {
                                    "query": {
                                        "match": {
                                            "appeals.content": {
                                                "query": keyword,
                                                "fuzziness": fuzziness,
                                                "prefix_length": 1
                                            }
                                        }
                                    },
                                    "script_score": {
                                        "script": {
                                            "source": "_score * 1.2 + 0.5",  # Boost fuzzy matches
                                            "lang": "painless"
                                        }
                                    },
                                    "boost": 1.0,
                                    "boost_mode": "multiply"
                                }
                            }
                        }
                    },
                    # Nested: comments.content
                    # {
                    #     "nested": {
                    #         "path": "comments",
                    #         "query": {
                    #             "match": {
                    #                 "comments.content": {
                    #                     "query": keyword,
                    #                     "fuzziness": 2,
                    #                     "prefix_length": 1
                    #                 }
                    #             }
                    #         }
                    #     }
                    # },
                    # # Nested: orders.name
                    # {
                    #     "nested": {
                    #         "path": "orders",
                    #         "query": {
                    #             "match": {
                    #                 "orders.name": {
                    #                     "query": keyword,
                    #                     "fuzziness": 2,
                    #                     "prefix_length": 1
                    #                 }
                    #             }
                    #         }
                    #     }
                    # },
                    # # Nested: orders.desc
                    # {
                    #     "nested": {
                    #         "path": "orders",
                    #         "query": {
                    #             "match": {
                    #                 "orders.desc": {
                    #                     "query": keyword,
                    #                     "fuzziness": 2,
                    #                     "prefix_length": 1
                    #                 }
                    #             }
                    #         }
                    #     }
                    # },
                    # # Nested: orders.others
                    # {
                    #     "nested": {
                    #         "path": "orders",
                    #         "query": {
                    #             "match": {
                    #                 "orders.others": {
                    #                     "query": keyword,
                    #                     "fuzziness": 2,
                    #                     "prefix_length": 1
                    #                 }
                    #             }
                    #         }
                    #     }
                    # },
                    # order_detail.note (if exists)
                    # {
                    #     "match": {
                    #         "order_detail.note": {
                    #             "query": keyword,
                    #             "fuzziness": 2,
                    #             "prefix_length": 1
                    #         }
                    #     }
                    # }
                        ],
                        "tie_breaker": 0.0  # Use maximum score only (0.0 = no tie-breaking)
                    }
                }
            ]
        }
    
    # Add problem_type filter if specified
    if problem_type_filter is not None:
        bool_query["filter"] = [
            {
                "term": {
                    "problem_type": problem_type_filter
                }
            }
        ]
    
    query = {
        "query": {
            "bool": bool_query
        },
        "min_score": min_score,  # Filter out documents with score below this threshold
        "size": 15,
        "highlight": {
            "fields": {
                "user_review": {},
                "others": {},
                "replies.content": {},
                "appeals.content": {},
                "comments.content": {},
                "orders.name": {},
                "orders.desc": {},
                "orders.others": {},
                "order_detail.note": {}
            },
            "pre_tags": ["<mark>"],
            "post_tags": ["</mark>"]
        }
    }
    
    print(f"\n🔍 Searching for: '{keyword}' (length: {len(keyword)} chars)")
    print(f"📊 Fuzziness: {fuzziness} (allows up to {fuzziness} character difference(s))")
    if problem_type_filter is not None:
        print(f"📊 Problem Type Filter: {problem_type_filter} (only showing type {problem_type_filter})")
    print(f"📊 Minimum score threshold: {min_score:.2f} (low-scoring results filtered)\n")
    
    try:
        response = es.search(index=index_name, body=query)
        total = response['hits']['total']['value']
        
        # Count how many were actually returned (after min_score filter)
        returned = len(response['hits']['hits'])
        
        print(f"Found {total} document(s) (showing {returned} above score {min_score:.2f})")
        
        # Show score statistics if we have results
        if returned > 0:
            scores = [hit['_score'] for hit in response['hits']['hits']]
            max_score = max(scores)
            min_score_found = min(scores)
            avg_score = sum(scores) / len(scores)
            print(f"Score range: {min_score_found:.2f} - {max_score:.2f} (avg: {avg_score:.2f})")
        
        print("\n" + "="*80)
        
        for i, hit in enumerate(response['hits']['hits'], 1):
            doc = hit['_source']
            print(f"\n[{i}] Score: {hit['_score']:.2f}")
            print(f"    ID: {doc.get('mongo_id')}")
            print(f"    Type: {doc.get('problem_type')}")
            
            # Show review if exists
            user_review = doc.get('user_review', '')
            if user_review:
                print(f"    Review: {user_review[:150]}...")
            
            # Show highlights with field names
            if 'highlight' in hit and hit['highlight']:
                print("    💡 Matches found in:")
                for field, highlights in hit['highlight'].items():
                    # Clean up field names for display
                    # field_display = field.replace('replies.', '').replace('appeals.', '').replace('comments.', '').replace('orders.', '').replace('order_detail.', '')
                    field_display = field
                    for highlight in highlights:
                        # Remove HTML tags for cleaner display
                        clean_highlight = highlight.replace('<mark>', '').replace('</mark>', '')
                        print(f"      • {field_display}: ...{clean_highlight[:100]}...")
            else:
                # If no highlights but document matched, show what we can
                if user_review:
                    print(f"    (Matched in user_review)")
                elif doc.get('others'):
                    print(f"    (Matched in others: {doc.get('others')[:100]}...)")
                else:
                    print(f"    (Matched in nested fields)")
        
        print("\n" + "="*80)
        
        # Show helpful message if no results
        if returned == 0:
            print("\n💡 No results found above the minimum score threshold.")
            print(f"   Try lowering the threshold: python quick_search.py \"{keyword}\" {min_score * 0.5:.1f}")
            print("   Or remove threshold: python quick_search.py \"{keyword}\" 0.0")
        
    except (TransportError, RequestError) as e:
        print(f"✗ Search error: {e}")


if __name__ == "__main__":
    main()
