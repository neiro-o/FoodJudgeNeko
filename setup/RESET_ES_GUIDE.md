# Reset Elasticsearch Index Guide

## Quick Start

### Full Reset (Delete + Recreate)

```bash
cd setup
python reset_es_index.py
```

This will:
1. ✅ Delete the existing `problems` index (if it exists)
2. ✅ Recreate it with proper mappings and settings
3. ✅ Show next steps for syncing data

### Delete Only

```bash
python reset_es_index.py --delete
```

### Create Only

```bash
python reset_es_index.py --create
```

## Complete Reset Workflow

### Step 1: Reset the Index

```bash
cd setup
python reset_es_index.py
```

Output:
```
================================================================================
RESETTING ELASTICSEARCH INDEX
================================================================================

Index: problems

[1/2] Deleting existing index...
⚠️  Index 'problems' contains 2,667 documents
✓ Deleted index 'problems'

[2/2] Creating new index...
✓ Created index 'problems' with proper mappings

================================================================================
✅ Index reset complete!
================================================================================

Next steps:
  1. Run sync script to populate index:
     cd ../ESsync && python sync_to_es.py
  2. Verify document count:
     python count_docs.py
```

### Step 2: Sync Data from MongoDB

```bash
cd ../ESsync
python sync_to_es.py
```

### Step 3: Verify

```bash
python count_docs.py
```

## Options

### Custom Config Path

```bash
python reset_es_index.py --config /path/to/config.yml
```

### Delete Only (No Recreate)

```bash
python reset_es_index.py --delete
```

Useful when you want to:
- Clean up before recreating manually
- Delete without recreating immediately

### Create Only (Skip if Exists)

```bash
python reset_es_index.py --create
```

Useful when:
- Index was accidentally deleted
- You only want to create (won't delete existing)

## What Gets Reset

### Deleted
- ✅ All documents in the index
- ✅ Index mappings
- ✅ Index settings

### Recreated
- ✅ Index with proper mappings (from `es_init.py`)
- ✅ IK Analyzer settings for Chinese text
- ✅ Nested field mappings (replies, appeals, orders, comments)
- ✅ Text field analyzers (ik_max_word, ik_smart)

## Index Mappings

The reset script uses mappings from `es_init.py` which include:

- **Text fields** with IK Analyzer:
  - `user_review`
  - `content` (in replies, appeals, comments)
  - `others`
  - `orders.name`, `orders.desc`

- **Nested fields**:
  - `replies[]` - Reply timeline
  - `appeals[]` - Appeals/evidence
  - `orders[]` - Order items
  - `comments[]` - Evaluation comments

- **Keyword fields**:
  - `mongo_id`
  - `problem_type`
  - `taskId`, `userId`

- **Numeric fields**:
  - `timestamp`, `created_at`
  - `stars`, `answer`
  - `ratio_1`, `ratio_2`

## Safety Features

### Document Count Warning

Before deletion, the script shows:
```
⚠️  Index 'problems' contains 2,667 documents
```

### Existence Check

- Won't fail if index doesn't exist
- Won't recreate if index already exists (unless using `--delete` first)

### Connection Check

- Verifies Elasticsearch connection before proceeding
- Shows clear error messages if connection fails

## Common Use Cases

### 1. Fresh Start

```bash
# Reset everything
cd setup
python reset_es_index.py

# Sync all data
cd ../ESsync
python sync_to_es.py
```

### 2. Fix Mapping Issues

If mappings are incorrect:

```bash
cd setup
python reset_es_index.py  # Reset with correct mappings
cd ../ESsync
python sync_to_es.py      # Re-index all data
```

### 3. Clean Up Test Data

```bash
cd setup
python reset_es_index.py --delete  # Delete test index
```

### 4. Recreate After Accidental Deletion

```bash
cd setup
python reset_es_index.py --create  # Recreate index
cd ../ESsync
python sync_to_es.py               # Re-index data
```

## Troubleshooting

### Error: Index Not Found

If you see:
```
ℹ️  Index 'problems' does not exist (nothing to delete)
```

This is normal if the index was already deleted. The script will continue to create it.

### Error: IK Analyzer Not Found

If you see:
```
✗ IK Analyzer plugin is NOT installed
```

You need to install the IK Analyzer plugin. See `ES_CHINESE_SETUP.md` for instructions.

### Error: Connection Failed

If you see:
```
✗ Cannot connect to Elasticsearch
```

Check:
1. Elasticsearch is running
2. Connection settings in `config.yml`
3. Username/password are correct
4. Network/firewall settings

## Integration with Sync Script

After resetting, always run the sync script:

```bash
# Reset index
cd setup
python reset_es_index.py

# Sync data
cd ../ESsync
python sync_to_es.py

# Verify
python count_docs.py
```

## Example Session

```bash
$ cd setup
$ python reset_es_index.py

Connecting to Elasticsearch...
✓ Connected to Elasticsearch

================================================================================
RESETTING ELASTICSEARCH INDEX
================================================================================

Index: problems

[1/2] Deleting existing index...
⚠️  Index 'problems' contains 2,667 documents
✓ Deleted index 'problems'

[2/2] Creating new index...
✓ Created index 'problems' with proper mappings

================================================================================
✅ Index reset complete!
================================================================================

Next steps:
  1. Run sync script to populate index:
     cd ../ESsync && python sync_to_es.py
  2. Verify document count:
     python count_docs.py

$ cd ../ESsync
$ python sync_to_es.py
[... sync output ...]

$ python count_docs.py
Total Documents: 2,667
```

---

**Location:** `setup/reset_es_index.py`  
**Dependencies:** `es_init.py`, `config.yml`  
**Status:** ✅ Ready to Use
