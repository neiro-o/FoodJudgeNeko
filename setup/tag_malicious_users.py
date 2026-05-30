#!/usr/bin/env python3
"""
Tag malicious users based on keyword frequency in comments.

Reads keywords and thresholds from a text file (format: keyword,count per line).
For each user, counts how many times each keyword appears in their comments.
If any keyword appears more than the threshold, tags the user as malicious.
"""

import sys
import os
from pathlib import Path
from typing import Dict, List, Tuple
from pymongo import MongoClient, ASCENDING
from pymongo.collection import Collection
from pymongo.database import Database
from pymongo.errors import OperationFailure
import argparse
import yaml


KeywordRule = Tuple[str, int, bool]
EXCEPTION_USER_IDS = {"504249549", "111", "35440012"}


def load_config():
    """
    Load configuration from crawlv2/config.yml
    
    Returns:
        Configuration dictionary.
    """
    # Try to find config.yml in crawlv2 directory
    script_dir = Path(__file__).parent
    config_path = script_dir.parent / "crawlv2" / "config.yml"
    
    if not config_path.exists():
        # Fallback to config.yml in parent directory
        config_path = script_dir.parent / "config.yml"
        if not config_path.exists():
            raise FileNotFoundError("config.yml not found. Expected at crawlv2/config.yml or config.yml")
    
    with open(config_path, 'r', encoding='utf-8') as f:
        return yaml.safe_load(f)


def build_mongo_connection_string(config: Dict) -> str:
    """
    Build MongoDB connection string from config.
    
    Args:
        config: Configuration dictionary.
    
    Returns:
        MongoDB connection string.
    """
    mongo_cfg = config['mongodb']
    host = mongo_cfg.get('host', '127.0.0.1')
    port = mongo_cfg.get('port', 27017)
    username = mongo_cfg.get('username', '')
    password = mongo_cfg.get('password', '')
    database = mongo_cfg.get('database', 'mtv2')
    
    if username and password:
        return f"mongodb://{username}:{password}@{host}:{port}/{database}"
    else:
        return f"mongodb://{host}:{port}/{database}"


def load_keywords(file_path: str) -> List[KeywordRule]:
    """
    Load keywords, counts, and case sensitivity from a text file.
    
    Format: keyword,count[,case_mode] (one per line)
    case_mode is optional and defaults to case-insensitive.
    Example:
        keyword1,5
        keyword2,10,i
        Keyword3,3,s
    
    Args:
        file_path: Path to the keywords file.
    
    Returns:
        List of (keyword, count, case_sensitive) tuples.
    """
    keywords = []
    if not os.path.exists(file_path):
        print(f"Error: Keywords file not found: {file_path}")
        return keywords
    
    with open(file_path, 'r', encoding='utf-8') as f:
        for line_num, line in enumerate(f, 1):
            line = line.strip()
            if not line or line.startswith('#'):
                continue
            
            try:
                parts = line.split(',')
                if len(parts) not in (2, 3):
                    print(f"Warning: Line {line_num} has invalid format, skipping: {line}")
                    continue
                
                keyword = parts[0].strip()
                count = int(parts[1].strip())
                case_mode = parts[2].strip().lower() if len(parts) == 3 else "i"
                if case_mode in ("i", "insensitive", "case-insensitive", "ignorecase"):
                    case_sensitive = False
                elif case_mode in ("s", "sensitive", "case-sensitive", "case_sensitive"):
                    case_sensitive = True
                else:
                    print(f"Warning: Line {line_num} has invalid case mode, skipping: {line}")
                    continue
                
                if keyword:
                    keywords.append((keyword, count, case_sensitive))
            except ValueError as e:
                print(f"Warning: Line {line_num} has invalid count, skipping: {line} ({e})")
                continue
    
    return keywords


def ensure_malicious_collection(db: Database, malicious_collection_name: str) -> None:
    """
    Ensure the malicious collection exists and has proper indexes.
    
    Args:
        db: MongoDB database instance.
        malicious_collection_name: Name of the malicious collection.
    """
    collection = db[malicious_collection_name]
    
    print(f"Setting up malicious collection ({malicious_collection_name})...")
    
    # Check if collection exists by trying to get its count
    try:
        collection.count_documents({})
        print(f"✓ Collection '{malicious_collection_name}' already exists")
    except Exception:
        # Collection doesn't exist, create it by inserting and deleting a dummy document
        print(f"  Creating collection '{malicious_collection_name}'...")
        try:
            collection.insert_one({"_temp": True})
            collection.delete_one({"_temp": True})
            print(f"✓ Collection '{malicious_collection_name}' created")
        except Exception as e:
            print(f"⚠ Warning creating collection: {e}")
    
    # Create indexes
    index_configs = [
        ([("userId", ASCENDING)], {"unique": True}),  # userId should be unique
        ([("tagged_at", ASCENDING)], {}),  # For sorting by tag date
    ]
    
    try:
        for index_fields, index_options in index_configs:
            collection.create_index(index_fields, **index_options)
        print(f"✓ Malicious collection indexes verified/created")
    except OperationFailure as e:
        print(f"⚠ Warning creating malicious indexes: {e}")
    except Exception as e:
        print(f"⚠ Error creating malicious indexes: {e}")


def count_keyword_in_comments(
    comments_collection: Collection,
    user_id: str,
    keyword: str,
    case_sensitive: bool
) -> int:
    """
    Count how many times a keyword appears in a user's comments.
    
    Args:
        comments_collection: MongoDB comments collection.
        user_id: User ID to search for.
        keyword: Keyword to count.
        case_sensitive: Whether uppercase/lowercase must match exactly.
    
    Returns:
        Number of comments containing the keyword.
    """
    regex_query = {"$regex": keyword}
    if not case_sensitive:
        regex_query["$options"] = "i"

    # Count documents where userId matches and content contains the keyword
    count = comments_collection.count_documents({
        "userId": user_id,
        "content": regex_query
    })
    if count > 0:
        print(user_id, count)
    return count


def tag_malicious_users(
    db: Database,
    keywords: List[KeywordRule],
    config: Dict
) -> None:
    """
    Tag malicious users based on keyword frequency.
    
    Args:
        db: MongoDB database instance.
        keywords: List of (keyword, threshold, case_sensitive) tuples.
        config: Configuration dictionary.
    """
    # Get collection names from config
    collections = config['mongodb'].get('collections', {})
    comments_collection_name = collections.get('comments', 'comments')
    malicious_collection_name = collections.get('malicious', 'malicious') # Not in config, use default
    
    # Ensure malicious collection exists with proper indexes
    ensure_malicious_collection(db, malicious_collection_name)
    print()
    
    comments_collection = db[comments_collection_name]
    malicious_collection = db[malicious_collection_name]
    
    print(f"Processing keywords: {len(keywords)}")
    for keyword, threshold, case_sensitive in keywords:
        case_mode = "case-sensitive" if case_sensitive else "case-insensitive"
        print(f"  - '{keyword}' (threshold: {threshold}, {case_mode})")
    print()
    
    # Get all unique user IDs from comments
    print("Fetching unique user IDs from comments collection...")
    user_ids = comments_collection.distinct("userId")
    print(f"Found {len(user_ids)} unique users")
    print()
    
    # Process each user
    tagged_count = 0
    already_tagged = 0
    exception_count = 0
    checked_count = 0
    
    for idx, user_id in enumerate(user_ids, 1):
        if idx % 100 == 0:
            print(f"Processing user {idx}/{len(user_ids)}...")

        if str(user_id) in EXCEPTION_USER_IDS:
            exception_count += 1
            continue
        
        # Check if user is already tagged
        existing = malicious_collection.find_one({"userId": user_id})
        if existing:
            already_tagged += 1
            continue
        
        # Check each keyword
        is_malicious = False
        matched_keywords = []
        
        for keyword, threshold, case_sensitive in keywords:
            count = count_keyword_in_comments(comments_collection, user_id, keyword, case_sensitive)
            if count >= threshold:
                is_malicious = True
                matched_keywords.append({
                    "keyword": keyword,
                    "count": count,
                    "threshold": threshold,
                    "case_sensitive": case_sensitive
                })
        
        if is_malicious:
            # Tag user as malicious
            malicious_collection.replace_one(
                {"userId": user_id},
                {
                    "userId": user_id,
                    "tagged": True,
                    "matched_keywords": matched_keywords,
                    "tagged_at": __import__('datetime').datetime.utcnow()
                },
                upsert=True
            )
            tagged_count += 1
            if tagged_count <= 10:  # Show first 10 tagged users
                print(f"  Tagged user {user_id}: {matched_keywords}")
        
        checked_count += 1
    
    print()
    print(f"Summary:")
    print(f"  Total users checked: {checked_count}")
    print(f"  Exceptions skipped: {exception_count}")
    print(f"  Already tagged: {already_tagged}")
    print(f"  Newly tagged: {tagged_count}")
    print(f"  Total malicious users: {malicious_collection.count_documents({})}")


def main():
    parser = argparse.ArgumentParser(description='Tag malicious users based on keyword frequency')
    parser.add_argument(
        'keywords_file',
        help='Path to keywords file (format: keyword,count[,case_mode] per line)'
    )
    parser.add_argument(
        '--config',
        help='Path to config.yml file (default: ../config.yml)'
    )
    
    args = parser.parse_args()
    
    # Load config
    try:
        if args.config:
            config_path = Path(args.config)
        else:
            script_dir = Path(__file__).parent
            config_path = script_dir.parent / "config.yml"
        
        if not config_path.exists():
            raise FileNotFoundError(f"Config file not found: {config_path}")
        
        print(f"Loading configuration from: {config_path}")
        with open(config_path, 'r', encoding='utf-8') as f:
            config = yaml.safe_load(f)
    except Exception as e:
        print(f"Error loading config: {e}")
        sys.exit(1)
    
    # Load keywords
    print(f"Loading keywords from: {args.keywords_file}")
    keywords = load_keywords(args.keywords_file)
    
    if not keywords:
        print("Error: No valid keywords found in file")
        sys.exit(1)
    
    # Build connection string from config
    connection_string = build_mongo_connection_string(config)
    database_name = config['mongodb'].get('database', 'mtv2')
    
    # Connect to MongoDB
    print(f"Connecting to MongoDB: {connection_string}")
    try:
        client = MongoClient(connection_string, serverSelectionTimeoutMS=5000)
        client.server_info()  # Test connection
        print("Connected successfully")
    except Exception as e:
        print(f"Error connecting to MongoDB: {e}")
        sys.exit(1)
    
    db = client[database_name]
    print(f"Using database: {database_name}")
    
    # Get collection names
    collections = config['mongodb'].get('collections', {})
    comments_collection_name = collections.get('comments', 'comments')
    print(f"Using comments collection: {comments_collection_name}")
    print(f"Using malicious collection: malicious")
    print()
    
    # Tag malicious users
    try:
        tag_malicious_users(
            db,
            keywords,
            config
        )
    except Exception as e:
        print(f"Error processing users: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
    finally:
        client.close()


if __name__ == "__main__":
    main()
