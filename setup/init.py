#!/usr/bin/env python3
"""
MongoDB initialization script for mtv2 backend.
Creates indexes and initial admin user.
"""

import sys
import yaml
import getpass
from datetime import datetime
from pymongo import MongoClient, ASCENDING, DESCENDING
from pymongo.errors import OperationFailure
import bcrypt


def load_config():
    """Load configuration from config.yml"""
    import os
    from pathlib import Path
    
    # Try to find config.yml in parent directory or current directory
    config_path = Path(__file__).parent.parent / "config.yml"
    if not config_path.exists():
        config_path = Path("config.yml")
        if not config_path.exists():
            raise FileNotFoundError("config.yml not found")
    
    with open(config_path, 'r') as f:
        return yaml.safe_load(f)


def hash_password(password: str) -> str:
    """Hash a password using bcrypt"""
    salt = bcrypt.gensalt()
    hashed = bcrypt.hashpw(password.encode('utf-8'), salt)
    return hashed.decode('utf-8')


def create_indexes(db, config):
    """Create indexes for accounts and invitations collections"""
    accounts_collection = db[config['mongodb']['collections']['accounts']]
    invitations_collection = db[config['mongodb']['collections']['invitations']]
    
    print("Creating indexes for accounts collection...")
    try:
        # Create indexes for accounts
        accounts_collection.create_index([("username", ASCENDING)], unique=True)
        accounts_collection.create_index([("email", ASCENDING)], unique=True)
        accounts_collection.create_index([("created_at", DESCENDING)])
        print("✓ Accounts indexes created successfully")
    except OperationFailure as e:
        print(f"⚠ Warning creating accounts indexes: {e}")
    
    print("Creating indexes for invitations collection...")
    try:
        # Create indexes for invitations
        invitations_collection.create_index([("invite_code", ASCENDING)], unique=True)
        invitations_collection.create_index([("created_by", ASCENDING)])
        invitations_collection.create_index([("created_at", DESCENDING)])
        invitations_collection.create_index([("used", ASCENDING)])
        print("✓ Invitations indexes created successfully")
    except OperationFailure as e:
        print(f"⚠ Warning creating invitations indexes: {e}")


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


def main():
    print("=" * 60)
    print("MongoDB Initialization Script for mtv2 Backend")
    print("=" * 60)
    print()
    
    # Load configuration
    try:
        config = load_config()
        print(f"✓ Configuration loaded from config.yml")
    except Exception as e:
        print(f"✗ Failed to load config: {e}")
        sys.exit(1)
    
    # Connect to MongoDB
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
        sys.exit(1)
    
    db = client[database_name]
    print(f"✓ Using database: {database_name}")
    print()
    
    # Create indexes
    create_indexes(db, config)
    print()
    
    # Create admin user
    print("=" * 60)
    print("Initial Admin User Setup")
    print("=" * 60)
    
    username = input("Enter admin username: ").strip()
    if not username:
        print("✗ Username cannot be empty")
        sys.exit(1)
    
    email = input("Enter admin email: ").strip()
    if not email:
        print("✗ Email cannot be empty")
        sys.exit(1)
    
    password = getpass.getpass("Enter admin password: ")
    if not password:
        print("✗ Password cannot be empty")
        sys.exit(1)
    
    password_confirm = getpass.getpass("Confirm admin password: ")
    if password != password_confirm:
        print("✗ Passwords do not match")
        sys.exit(1)
    
    print()
    create_admin_user(db, config, username, password, email)
    
    print()
    print("=" * 60)
    print("MongoDB initialization completed successfully!")
    print("=" * 60)
    
    client.close()


if __name__ == "__main__":
    main()

