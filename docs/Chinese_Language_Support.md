# Elasticsearch 中文语言支持

(这部分都是AI帮我写的，我也不知道对不对)

## 安装

### 1. 安装中文分析插件

中文语言支持需要安装 `analysis-smartcn` 插件。

```bash
# 安装中文分析插件
sudo bin/elasticsearch-plugin install analysis-smartcn

# 重启 Elasticsearch
sudo systemctl restart elasticsearch
```

### 2. 验证安装

```bash
# 检查已安装的插件
curl -X GET "localhost:9200/_cat/plugins?v"
```

## 配置

运行数据库初始化脚本会自动配置中文支持：

```bash
python3 deploy/init_databases.py
```

### 索引映射

系统会创建包含中文语言配置的 Elasticsearch 索引：

- **分析器**: `chinese_analyzer` - 使用 `smartcn_tokenizer` 进行中文分词
- **搜索分析器**: `chinese_search_analyzer` - 针对中文搜索优化
- **支持中文的字段**:
  - `user_review` - 用户评价内容
  - `replies.content` - 回复内容
  - `appeals.content` - 申诉内容
  - `others` - 其他文本内容

## 使用方法

配置完成后，中文文本会在索引和搜索时自动处理：

### 索引中文文本

系统会：
1. 使用 smartcn 分词器对中文文本进行分词
2. 应用小写和停用词过滤器
3. 存储处理后的词元用于高效搜索

### 搜索中文文本

用户使用中文搜索时，系统会：
1. 对搜索查询应用相同的分词处理
2. 与索引的中文词元进行匹配
3. 基于中文文本匹配返回相关结果

## 测试中文支持

### 索引中文文档
```bash
curl -X POST "localhost:9200/problems/_doc/1" -H 'Content-Type: application/json' -d'
{
  "user_review": "这个产品很好用，质量不错",
  "replies": [
    {
      "content": "谢谢您的反馈，我们会继续改进"
    }
  ]
}'
```

### 中文搜索
```bash
curl -X GET "localhost:9200/problems/_search" -H 'Content-Type: application/json' -d'
{
  "query": {
    "multi_match": {
      "query": "产品",
      "fields": ["user_review", "replies.content"]
    }
  }
}'
```

## 故障排除

### 插件安装问题

1. **检查 Elasticsearch 版本兼容性**:
   ```bash
   bin/elasticsearch --version
   ```

2. **验证插件可用性**:
   ```bash
   bin/elasticsearch-plugin list
   ```

### 搜索不工作

1. **验证插件已安装**:
   ```bash
   curl -X GET "localhost:9200/_cat/plugins"
   ```

2. **检查索引映射**:
   ```bash
   curl -X GET "localhost:9200/problems/_mapping"
   ```

3. **测试分析器**:
   ```bash
   curl -X POST "localhost:9200/_analyze" -H 'Content-Type: application/json' -d'
   {
     "analyzer": "chinese_analyzer",
     "text": "这是一个测试"
   }'
   ```

## 性能考虑

- 中文文本分析比英文更消耗资源
- 大量中文文本可能影响索引性能
- 监控 Elasticsearch 集群处理中文内容的性能
