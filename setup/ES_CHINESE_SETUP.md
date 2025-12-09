# Elasticsearch 8 + IK Analyzer Setup

## Installation

1. **Find your ES version:**
   ```bash
   curl http://localhost:9200
   ```

2. **Download IK Analyzer:**
   ```bash
   # For ES 8.11.0:
   wget https://github.com/medcl/elasticsearch-analysis-ik/releases/download/v8.11.0/elasticsearch-analysis-ik-8.11.0.zip
   
   # For other versions, check:
   https://github.com/medcl/elasticsearch-analysis-ik/releases
   ```

3. **Install plugin:**
   ```bash
   cd /path/to/elasticsearch
   bin/elasticsearch-plugin install file:///path/to/elasticsearch-analysis-ik-8.x.x.zip
   ```

4. **Restart Elasticsearch**

5. **Verify:**
   ```bash
   curl -X GET 'http://localhost:9200/_cat/plugins'
   ```

## Usage

```python
from setup.es_search_examples import search_by_keywords
from setup.es_init import init_elasticsearch
import yaml

# Initialize
with open('config.yml', 'r') as f:
    config = yaml.safe_load(f)
es = init_elasticsearch(config)

# Search
results = search_by_keywords(es, "problems", "退款问题")
print(f"Found {results['hits']['total']['value']} results")
```

## Search Functions

- `search_by_keywords()` - Simple search across all fields
- `search_nested()` - Search with nested queries for replies/appeals
- `search_with_filters()` - Search with filters (problem_type, answer)

## References

- [IK Analyzer GitHub](https://github.com/medcl/elasticsearch-analysis-ik)
