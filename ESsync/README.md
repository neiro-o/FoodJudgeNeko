# ESsync - MongoDB to Elasticsearch Converter

This module converts MongoDB BSON documents (Meituan evaluation data) to Elasticsearch document format.

## Overview

The converter supports multiple problem types:
- **Type 1** (外卖/Waimai - Takeaway reviews) - Full order and delivery details
- **Type 2** (堂食/Dine-in reviews) - Shop info and dining experience

The converters transform nested BSON data into a flattened, searchable Elasticsearch document structure.

## Features

- ✅ **Robust error handling**: Uses `safe_get()` to prevent KeyError crashes
- ✅ **Type conversion**: Automatically handles string-to-int, timestamp conversions (all timestamps as integers)
- ✅ **Default values**: Returns sensible defaults when fields are missing
- ✅ **Data validation**: Includes comprehensive test suite
- ✅ **Sorted replies**: Replies are automatically sorted by timestamp in ascending order

## Installation

No external dependencies required - uses only Python standard library.

```bash
# Simply copy the ESsync directory to your project
cp -r ESsync /path/to/your/project/
```

## Usage

### Basic Usage

**Type 1 (Takeaway/外卖):**
```python
from ESsync import convert_bson_to_es_type1
import json

# Load BSON document
with open('sample.json', 'r', encoding='utf-8') as f:
    bson_doc = json.load(f)

# Convert to ES format
es_doc = convert_bson_to_es_type1(bson_doc)

# Use the ES document
print(es_doc['stars'])  # 1.0
print(es_doc['problem_type'])  # 1
print(len(es_doc['orders']))  # 12
```

**Type 2 (Dine-in/堂食):**
```python
from ESsync import convert_bson_to_es_type2
import json

# Load BSON document
with open('sample2.json', 'r', encoding='utf-8') as f:
    bson_doc = json.load(f)

# Convert to ES format
es_doc = convert_bson_to_es_type2(bson_doc)

# Use the ES document
print(es_doc['stars'])  # 1.0
print(es_doc['problem_type'])  # 2
print(es_doc['order_info']['shop_info']['name'])  # Shop name
```

**Type 3 (Takeaway Refund/外卖退款):**
```python
from ESsync import convert_bson_to_es_type3
import json

# Load BSON document
with open('sample3.json', 'r', encoding='utf-8') as f:
    bson_doc = json.load(f)

# Convert to ES format
es_doc = convert_bson_to_es_type3(bson_doc)

# Use the ES document
print(es_doc['problem_type'])  # 3
print(es_doc['user_review'])  # "" (empty)
print(len(es_doc['orders']))  # Refund items
```

**Type 4 (Dine-in Refund/堂食退款):**
```python
from ESsync import convert_bson_to_es_type4
import json

# Load BSON document
with open('sample4.json', 'r', encoding='utf-8') as f:
    bson_doc = json.load(f)

# Convert to ES format
es_doc = convert_bson_to_es_type4(bson_doc)

# Use the ES document
print(es_doc['problem_type'])  # 4
print(es_doc['user_review'])  # "Refund for 38.8"
print(es_doc['timestamp'])  # First appeal timestamp
```

### Running Tests

**Type 1 Tests:**
```bash
cd ESsync
python test_converter.py
```

**Type 2 Tests:**
```bash
cd ESsync
python test_type2.py
```

**Type 3 Tests:**
```bash
cd ESsync
python test_type3.py
```

**Type 4 Tests:**
```bash
cd ESsync
python test_type4.py
```

Tests will:
- Load sample documents
- Convert to ES format
- Display formatted results
- Run validation checks
- Save output to JSON files

## Field Mappings

### Top-Level Fields

| ES Field | BSON Source | Transformation |
|----------|-------------|----------------|
| `mongo_id` | `_id` | Direct |
| `stars` | `detail.taskInfo.voteContent.review.star` | ÷ 10 |
| `user_review` | `detail.taskInfo.voteContent.review.reviewContent` | Direct |
| `review_pics` | `detail.taskInfo.voteContent.review.reviewPicList` | Array |
| `timestamp` | `detail.taskInfo.voteContent.review.addTime` | ms → s |
| `others` | `merchant.poiCategoryList` + `merchant.city` | Join + concat |
| `problem_type` | - | Always `1` |
| `answer` | `newSupportRatio` vs `newOpposeRatio` | 1 if oppose > support, else 2 |
| `ratio_1` | `detail.taskInfo.newOpposeRatio` | Int |
| `ratio_2` | `detail.taskInfo.newSupportRatio` | Int |
| `uploader` | `uploader` | Direct or null |
| `taskId` | `taskId` | Direct |
| `userId` | `userId` | Direct |
| `created_at` | `upload_timestamp` | Direct |

### Nested Arrays

#### `replies` (from `replyList`)

**Note:** Replies are automatically **sorted by timestamp** in ascending order.

```python
{
  "role": "merchant" | "user" | "others_{num}",  # Converted from replyType
  "timestamp": 1706648442,  # addTime / 1000 (as integer)
  "content": "reply text"
}
```

#### `appeals` (from `evidenceList`)

```python
{
  "role": "merchant" | "user",  # Based on belongType
  "timestamp": 1706661455,  # createTime / 1000 (as integer)
  "content": "evidence text",
  "pics": ["url1", "url2"]  # pictureInfoDtoList + videoInfoDtoList
}
```

#### `orders` (from `productInfoDtoList`)

```python
{
  "name": "Product Name",
  "count": 1,
  "desc": "Product description",
  "selection": ["option1", "option2"],  # attrValueList
  "pic": "http://...",
  "others": "spec, Minimum per order: 2"  # Combined
}
```

#### `comments` (from `comment[].pageContent[]`)

```python
{
  "userid": 3417484203,
  "name": "User Name",
  "content": "Comment text",
  "timestamp": 1707211828,  # createTime / 1000 (as integer)
  "choice": 1  # 1 if voteOperate == 'DOWN', else 2
}
```

### Order Detail

```python
{
  "order_started": 1706586633,  # orderTime / 1000 (as integer)
  "order_finished": 1706589574,  # finishTime / 1000 (as integer)
  "deliver_time": 1854,  # deliveryTimeTakeMs / 1000 (as integer), or -1
  "total_time": 2941,  # orderTimeTakeMs / 1000 (as integer)
  "deliver_by": "meituan" | "merchant" | "user" | "others",
  "note": "User note",
  "utensils": -1,  # tablewareCount if needTableware, else 0
  "invoice": false
}
```

### Special Conversions

#### Reply Role Mapping

| BSON `replyType` | ES `role` |
|------------------|-----------|
| `MERCHANT` | `merchant` |
| `CUSTOMER` | `user` |
| Other/null | `others_{index}` |

#### Delivery Type Mapping

| BSON `deliveryTypeCn` | ES `deliver_by` |
|-----------------------|-----------------|
| `美团配送` | `meituan` |
| `商家配送` | `merchant` |
| `顾客自取` / `用户自取` | `user` |
| Other | `others` |

## Error Handling

The converter uses the `safe_get()` helper function to gracefully handle missing keys:

```python
# Instead of this (crashes on missing keys):
value = data["detail"]["taskInfo"]["voteContent"]

# We use this (returns default if missing):
value = safe_get(data, "detail", "taskInfo", "voteContent", default=None)
```

**Benefits:**
- No `KeyError` exceptions
- Graceful degradation with default values
- Continues processing even with incomplete data

## Output Example

```json
{
  "mongo_id": "6890e909d48acea838884475",
  "stars": 1.0,
  "user_review": "能不能页面提示一下香油碟有香菜啊...",
  "review_pics": ["https://p0.meituan.net/..."],
  "timestamp": 1706591722,
  "others": "川渝火锅 in 北京市",
  "problem_type": 1,
  "answer": 1,
  "ratio_1": 51,
  "ratio_2": 49,
  "uploader": "43.134.5.122",
  "taskId": "QXdRQUFBQkpBZ...",
  "userId": "5563803811",
  "created_at": 1764763505,
  "replies": [...],
  "appeals": [...],
  "order_info": null,
  "orders": [...],
  "order_detail": {...},
  "comments": [...]
}
```

## File Structure

```
ESsync/
├── __init__.py           # Package initialization
├── converter.py          # Main conversion function
├── test_converter.py     # Test suite
├── test_output.json      # Test output (generated)
└── README.md            # This file
```

## Supported Types

The package includes an **auto-detection function** that automatically selects the right converter based on `processType`:

- ✅ **Type 1**: 外卖 (Waimai/Takeaway) - `WAIMAI_COMMENT`
- ✅ **Type 2**: 堂食 (Dine-in) - `DAODIAN_MEAL`, `STAR_COMMENT`, `MINSU_COMMENT`, `DAODIAN_HOTEL`, `DAODIAN_TICKET`, `DAODIAN_COMPLEX`
- ✅ **Type 3**: 外卖退款 (Takeaway refunds) - `WAIMAI_QUIK_REFUND`
- ✅ **Type 4**: 堂食退款 (Dine-in refunds) - `DAOZONG_JIAOYI`
- ⏳ **Type 5**: IPR - Reserved (not yet implemented)

### Auto-Detection

Use `convert_bson_to_es()` to automatically detect and convert any type:

```python
from ESsync import convert_bson_to_es

# Works with all types automatically!
es_doc = convert_bson_to_es(bson_doc)
```

### Manual Type Selection

Or use type-specific converters:
- `convert_bson_to_es_type1()` - Takeaway reviews
- `convert_bson_to_es_type2()` - Dine-in/service reviews
- `convert_bson_to_es_type3()` - Takeaway refunds
- `convert_bson_to_es_type4()` - Dine-in refunds

## Documentation

### Main Guides
- [Auto-Detection Guide](AUTO_DETECT_GUIDE.md) - **Start here!** Auto-detect type and convert
- [Type Comparison](ALL_TYPES_COMPARISON.md) - Compare all 4 types side-by-side

### Type-Specific Guides
- [Type 1 Guide](README.md) - This file covers Type 1 in detail
- [Type 2 Guide](TYPE2_GUIDE.md) - Complete guide for Type 2 converter
- [Type 3 Guide](TYPE3_GUIDE.md) - Complete guide for Type 3 converter (takeaway refunds)
- [Type 4 Guide](TYPE4_GUIDE.md) - Complete guide for Type 4 converter (dine-in refunds)

## License

Internal use only.

## Author

Created for MTV2 project - Meituan evaluation data processing.
