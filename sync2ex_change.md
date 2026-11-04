# Updates

## ESsync/sync_to_es.py

### Product minOrderCount 条件修改

行 248-249 和 272-273：条件从 `is not None` 改为 **数字且 > 1**

```python
# 改前：if p.get("minOrderCount") is not None
# 改后：if isinstance(min_order_count, (int, float)) and min_order_count > 1
```

只有最小起订数 > 1 时才显示 "Minimum per order"

### _review_base() - others 字段格式更新

增加 `poiName` 和 `poiAvgPrice` 字段到商户信息。

**格式示例：**
- 基础：`米粉/米线 (杭州市)`
- +店名：`米粉/米线 - XX店 (杭州市)`
- +人均价格：`米粉/米线 | 人均 45 元 (杭州市)`
- 完整：`米粉/米线 - XX店 | 人均 45 元 (杭州市)`
- 无城市：省略 `(city)` 部分

**条件：**
- `poiName`: 存在且非空时添加
- `poiAvgPrice`: 数字且 > 0 时添加
- `city`: 存在时始终包含在末尾括号
