#!/usr/bin/env python3
"""
Comment synchronization script for mtv2 backend.
Syncs comments from problems collection to comments collection.
"""

import sys
from pathlib import Path
from pymongo import MongoClient, ASCENDING
from pymongo.errors import OperationFailure
import yaml


def load_config():
    """Load configuration from config.yml"""
    config_path = Path(__file__).parent.parent / "config.yml"
    if not config_path.exists():
        config_path = Path("config.yml")
        if not config_path.exists():
            raise FileNotFoundError("config.yml not found")
    
    with open(config_path, 'r') as f:
        return yaml.safe_load(f)


def get_timestamp_file_path():
    """Get the path to the last sync timestamp file"""
    return Path(__file__).parent / "last_sync_cmt_timestamp"


def read_last_sync_timestamp():
    """Read the last sync timestamp from file, create with 0 if not exists"""
    timestamp_file = get_timestamp_file_path()
    
    if not timestamp_file.exists():
        print(f"✓ Timestamp file not found, creating with value 0")
        timestamp_file.write_text("0")
        return 0
    
    try:
        timestamp = int(timestamp_file.read_text().strip())
        print(f"✓ Last sync timestamp: {timestamp}")
        return timestamp
    except ValueError:
        print(f"⚠ Invalid timestamp file content, resetting to 0")
        timestamp_file.write_text("0")
        return 0


def write_last_sync_timestamp(timestamp):
    """Write the last sync timestamp to file"""
    timestamp_file = get_timestamp_file_path()
    timestamp_file.write_text(str(timestamp))
    print(f"✓ Updated last sync timestamp to: {timestamp}")


def ensure_comments_indexes(db, config):
    """Ensure indexes exist on comments collection for userId, problemId, commentId"""
    comments_collection_name = config['mongodb']['collections']['comments']
    collection = db[comments_collection_name]
    
    print(f"Checking indexes for comments collection ({comments_collection_name})...")
    
    index_configs = [
        ([("userId", ASCENDING)], {}),
        ([("problemId", ASCENDING)], {}),
        ([("commentId", ASCENDING)], {}),
    ]
    
    try:
        for index_fields, index_options in index_configs:
            collection.create_index(index_fields, **index_options)
        print(f"✓ Comments collection indexes verified/created")
    except OperationFailure as e:
        print(f"⚠ Warning creating comments indexes: {e}")
    except Exception as e:
        print(f"⚠ Error creating comments indexes: {e}")


def process_comment_item(item, problem_id, flag):
    """
    Process a single comment item.
    
    Args:
        item: The comment item from pageContent
        problem_id: The _id of the problem document (as string)
        flag: 1 if processType is DAOZONG_JIAOYI or IPR, 0 otherwise
    
    Returns:
        Processed comment dict ready for insertion
    """
    # Create a copy to avoid modifying original
    comment = dict(item)
    
    # Delete unwanted keys
    keys_to_delete = ['voteTaskNo', 'isMyComment', 'approveStatus', 'voteOperateDesc', 'userBasic']
    for key in keys_to_delete:
        comment.pop(key, None)
    
    # Convert createTime from milliseconds to seconds
    if 'createTime' in comment:
        try:
            comment['createTime'] = int(int(comment['createTime']) / 1000)
        except (ValueError, TypeError):
            pass
    
    # Add problemId
    comment['problemId'] = str(problem_id)
    
    # Determine choice based on voteOperate and flag
    vote_operate = item.get('voteOperate', '')
    
    # Normal case (flag=0): DOWN=1, UP=2
    # Reversed case (flag=1): DOWN=2, UP=1
    if vote_operate == 'DOWN':
        comment['choice'] = 2 if flag == 1 else 1
    elif vote_operate == 'UP':
        comment['choice'] = 1 if flag == 1 else 2
    else:
        comment['choice'] = 0  # Unknown or missing voteOperate
    
    # Remove voteOperate after processing (optional, keep if needed)
    # comment.pop('voteOperate', None)
    
    return comment


def sync_comments(config):
    """Main function to sync comments from problems to comments collection"""
    connection_string = config['mongodb']['connection_string']
    database_name = config['mongodb']['database_name']
    problems_collection_name = config['mongodb']['collections']['problems']
    comments_collection_name = config['mongodb']['collections']['comments']
    
    print(f"Connecting to MongoDB at {connection_string}...")
    try:
        client = MongoClient(connection_string, serverSelectionTimeoutMS=5000)
        client.server_info()
        print(f"✓ Connected to MongoDB successfully")
    except Exception as e:
        print(f"✗ Failed to connect to MongoDB: {e}")
        raise
    
    db = client[database_name]
    print(f"✓ Using database: {database_name}")
    print()
    
    # Step 0: Ensure indexes on comments collection
    ensure_comments_indexes(db, config)
    print()
    
    # Step 1: Read last sync timestamp
    last_sync_timestamp = read_last_sync_timestamp()
    print()
    
    # Step 2: Search problems where upload_timestamp > last_sync_timestamp
    problems_collection = db[problems_collection_name]
    comments_collection = db[comments_collection_name]
    
    query = {"upload_timestamp": {"$gt": last_sync_timestamp}}
    problems_cursor = problems_collection.find(query)
    
    total_comments_upserted = 0
    total_problems_processed = 0
    max_timestamp = last_sync_timestamp
    
    print(f"Searching for problems with upload_timestamp > {last_sync_timestamp}...")
    
    for problem in problems_cursor:
        problem_id = problem.get('_id')
        upload_timestamp = problem.get('upload_timestamp', 0)
        
        # Track the maximum timestamp for updating the file later
        if upload_timestamp > max_timestamp:
            max_timestamp = upload_timestamp
        
        # Step 3: Check processType to determine flag
        process_type = problem.get('detail', {}).get('taskInfo', {}).get('processType', '')
        flag = 1 if process_type in ['DAOZONG_JIAOYI', 'IPR'] else 0
        
        # Step 4: Find comment key and process all pageContent items
        comment_pages = problem.get('comment', [])
        
        if not comment_pages:
            continue
        
        comments_to_insert = []
        
        for page in comment_pages:
            page_content = page.get('pageContent', [])
            
            for item in page_content:
                # Step 5 & 6: Process each comment item
                processed_comment = process_comment_item(item, problem_id, flag)
                comments_to_insert.append(processed_comment)
        
        # Step 7: Upsert comments into comments collection (check by commentId)
        if comments_to_insert:
            upserted_count = 0
            for comment in comments_to_insert:
                comment_id = comment.get('commentId')
                if not comment_id:
                    continue
                try:
                    result = comments_collection.update_one(
                        {"commentId": comment_id},
                        {"$set": comment},
                        upsert=True
                    )
                    if result.upserted_id or result.modified_count > 0:
                        upserted_count += 1
                except Exception as e:
                    print(f"    ⚠ Failed to upsert comment {comment_id}: {e}")
            
            total_comments_upserted += upserted_count
            total_problems_processed += 1
            print(f"  ✓ Problem {problem_id}: upserted {upserted_count} comments (flag={flag})")
    
    print()
    print(f"✓ Sync completed:")
    print(f"  - Problems processed: {total_problems_processed}")
    print(f"  - Comments upserted: {total_comments_upserted}")
    
    # Update the timestamp file with the max timestamp found
    if max_timestamp > last_sync_timestamp:
        write_last_sync_timestamp(max_timestamp)
    else:
        print(f"✓ No new problems found, timestamp unchanged")
    
    client.close()
    return total_problems_processed, total_comments_upserted


def main():
    print("=" * 60)
    print("mtv2 Comment Synchronization Script")
    print("=" * 60)
    print()
    
    # Load configuration
    try:
        config = load_config()
        print(f"✓ Configuration loaded from config.yml")
    except Exception as e:
        print(f"✗ Failed to load config: {e}")
        sys.exit(1)
    
    print()
    
    try:
        problems_processed, comments_upserted = sync_comments(config)
    except Exception as e:
        print(f"✗ Comment sync failed: {e}")
        sys.exit(1)
    
    print()
    print("=" * 60)
    print("Sync Summary")
    print("=" * 60)
    print(f"✓ Problems processed: {problems_processed}")
    print(f"✓ Comments upserted: {comments_upserted}")
    print("=" * 60)


if __name__ == "__main__":
    main()
