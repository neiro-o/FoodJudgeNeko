"""
Simple usage example for the ESsync converter
"""

import json
from converter import convert_bson_to_es_type1


def main():
    # Example 1: Convert a single document
    print("Example 1: Converting a single BSON document")
    print("-" * 50)
    
    # Load BSON document from MongoDB export or JSON file
    with open('../sample.json', 'r', encoding='utf-8') as f:
        bson_doc = json.load(f)
    
    # Convert to ES format
    es_doc = convert_bson_to_es_type1(bson_doc)
    
    # Print some key fields
    print(f"Converted document ID: {es_doc['mongo_id']}")
    print(f"Stars: {es_doc['stars']}")
    print(f"Problem Type: {es_doc['problem_type']}")
    print(f"Number of orders: {len(es_doc['orders'])}")
    print(f"Number of comments: {len(es_doc['comments'])}")
    print()
    
    # Example 2: Batch conversion
    print("Example 2: Batch conversion of multiple documents")
    print("-" * 50)
    
    # Simulate multiple BSON documents
    bson_docs = [bson_doc]  # In practice, load from MongoDB cursor
    
    es_docs = []
    for doc in bson_docs:
        try:
            es_doc = convert_bson_to_es_type1(doc)
            es_docs.append(es_doc)
        except Exception as e:
            print(f"Failed to convert document: {e}")
    
    print(f"Successfully converted {len(es_docs)} documents")
    print()
    
    # Example 3: Integration with Elasticsearch (pseudo-code)
    print("Example 3: Elasticsearch integration (pseudo-code)")
    print("-" * 50)
    print("""
    from elasticsearch import Elasticsearch
    
    # Initialize ES client
    es = Elasticsearch(['http://localhost:9200'])
    
    # Convert and index document
    es_doc = convert_bson_to_es_type1(bson_doc)
    es.index(
        index='meituan_evaluations',
        id=es_doc['mongo_id'],
        body=es_doc
    )
    """)
    
    # Example 4: Safe field access
    print("\nExample 4: Safe field access with default values")
    print("-" * 50)
    
    from converter import safe_get
    
    # Even if nested keys don't exist, no KeyError is raised
    value = safe_get(bson_doc, "detail", "nonexistent", "key", default="N/A")
    print(f"Non-existent field value: {value}")
    
    # Works with deeply nested structures
    star = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "review", "star")
    print(f"Star rating from nested path: {star}")
    

if __name__ == "__main__":
    main()
