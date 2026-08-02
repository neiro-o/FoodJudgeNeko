#!/usr/bin/env python3
"""
MongoDB initialization functions for mtv2 backend.
Creates indexes and initial admin user.
"""

import sys
from datetime import datetime
from pymongo import MongoClient, ASCENDING, DESCENDING
from pymongo.errors import OperationFailure
import bcrypt


def hash_password(password: str) -> str:
    """Hash a password using bcrypt"""
    salt = bcrypt.gensalt()
    hashed = bcrypt.hashpw(password.encode('utf-8'), salt)
    return hashed.decode('utf-8')


def create_weekly_scores_collection(db, config):
    """Create the weekly_scores collection with its JSON schema validator (if missing)"""
    collection_name = config['mongodb']['collections'].get('weekly_scores', 'weekly_scores')

    if collection_name in db.list_collection_names():
        print(f"⚠ Collection '{collection_name}' already exists, skipping schema creation...")
        return

    validator = {
        "$jsonSchema": {
            "bsonType": "object",
            "required": [
                "userId",
                "year",
                "weekId",
                "score",
                "createdAt",
                "updatedAt",
            ],
            "properties": {
                "userId": {
                    "bsonType": "objectId",
                    "description": "必须对应 accounts._id",
                },
                "year": {
                    "bsonType": "int",
                    "minimum": 2000,
                },
                "weekId": {
                    "bsonType": "int",
                    "minimum": 1,
                    "maximum": 53,
                },
                "score": {
                    "bsonType": ["int", "long", "double", "decimal"],
                    "minimum": 0,
                },
                "createdAt": {
                    "bsonType": "date",
                },
                "updatedAt": {
                    "bsonType": "date",
                },
            },
        }
    }

    try:
        db.create_collection(
            collection_name,
            validator=validator,
            validationLevel="strict",
            validationAction="error",
        )
        print(f"✓ Collection '{collection_name}' created with schema validation")
    except OperationFailure as e:
        print(f"⚠ Warning creating '{collection_name}' collection: {e}")


def create_indexes(db, config):
    """Create indexes for all collections defined in config.yml"""
    collections = config['mongodb']['collections']
    
    # Define index configurations for each collection
    index_configs = {
        'accounts': [
            ([("username", ASCENDING)], {"unique": True}),
            ([("email", ASCENDING)], {"unique": True}),
            ([("created_at", DESCENDING)], {}),
        ],
        'invitations': [
            ([("invite_code", ASCENDING)], {"unique": True}),
            ([("created_by", ASCENDING)], {}),
            ([("created_at", DESCENDING)], {}),
            ([("used", ASCENDING)], {}),
        ],
        'processed_list': [
            ([("user_id", ASCENDING), ("task_id", ASCENDING)], {"unique": True}),
            ([("created_at", DESCENDING)], {}),
            ([("user_id", ASCENDING)], {}),
        ],
        'problems': [
            # Matches the exact filter used by UploadProblem/UploadMultipleProblems
            # (backend/handlers/problem.go) to check for existing userId+taskId
            # combos before queueing an upload. Without this compound index, that
            # duplicate check falls back to scanning via the single-field taskId
            # index (or a full collection scan), and the lack of a unique
            # constraint means concurrent uploads can still race past the
            # app-level check and insert duplicate problems.
            ([("userId", ASCENDING), ("taskId", ASCENDING)], {"unique": True}),
            ([("uploader", ASCENDING)], {}),
            ([("taskId", ASCENDING)], {}),
            ([("created_at", DESCENDING)], {}),
            ([("timestamp", DESCENDING)], {}),
        ],
        'notes': [
            # ([("created_at", DESCENDING)], {}),
        ],
        'ai_user_summaries': [
            ([("userId", ASCENDING)], {"unique": True}),
            ([("generatedAt", DESCENDING)], {}),
        ],
        'weekly_scores': [
            ([("userId", ASCENDING), ("year", ASCENDING), ("weekId", ASCENDING)], {"unique": True}),
            ([("year", ASCENDING), ("weekId", ASCENDING), ("score", DESCENDING)], {}),
        ],
    }
    
    # Create indexes for each collection in config
    for collection_key, collection_name in collections.items():
        if collection_key not in index_configs:
            # Collection exists in config but no index config defined
            print(f"⚠ No index configuration for '{collection_key}' collection, skipping...")
            continue
        
        collection = db[collection_name]
        print(f"Creating indexes for {collection_key} collection ({collection_name})...")
        
        try:
            for index_fields, index_options in index_configs[collection_key]:
                collection.create_index(index_fields, **index_options)
            print(f"✓ {collection_key.capitalize()} indexes created successfully")
        except OperationFailure as e:
            print(f"⚠ Warning creating {collection_key} indexes: {e}")
        except Exception as e:
            print(f"⚠ Error creating {collection_key} indexes: {e}")


def create_admin_user(db, config, username: str, password: str, email: str):
    """Create initial admin user"""
    accounts_collection = db[config['mongodb']['collections']['accounts']]
    
    # Check if admin user already exists
    existing = accounts_collection.find_one({"username": username})
    if existing:
        print(f"⚠ User '{username}' already exists. Skipping admin creation.")
        return False
    
    # Check if email already exists
    existing_email = accounts_collection.find_one({"email": email})
    if existing_email:
        print(f"⚠ Email '{email}' already exists. Skipping admin creation.")
        return False
    
    # Hash password
    hashed_password = hash_password(password)
    
    # Create admin account
    admin_account = {
        "username": username,
        "password": hashed_password,
        "email": email,
        "points": 0,
        "is_admin": True,
        "created_at": datetime.utcnow()
    }
    
    result = accounts_collection.insert_one(admin_account)
    print(f"✓ Admin user '{username}' created successfully with ID: {result.inserted_id}")
    return True


def init_mongodb(config):
    """Initialize MongoDB: create indexes and optionally create admin user"""
    connection_string = config['mongodb']['connection_string']
    database_name = config['mongodb']['database_name']
    
    print(f"Connecting to MongoDB at {connection_string}...")
    try:
        client = MongoClient(connection_string, serverSelectionTimeoutMS=5000)
        # Test connection
        client.server_info()
        print(f"✓ Connected to MongoDB successfully")
    except Exception as e:
        print(f"✗ Failed to connect to MongoDB: {e}")
        raise
    
    db = client[database_name]
    print(f"✓ Using database: {database_name}")
    print()
    
    # Show all collections from config
    collections = config['mongodb']['collections']
    print(f"Collections to initialize: {', '.join([f'{k} ({v})' for k, v in collections.items()])}")
    print()
    
    # Create the weekly_scores collection with its schema validator, if missing
    create_weekly_scores_collection(db, config)
    print()

    # Create indexes for all collections
    create_indexes(db, config)
    print()
    
    return client, db

