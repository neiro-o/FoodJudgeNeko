#!/usr/bin/env python3
"""
Main initialization script for mtv2 backend.
Orchestrates MongoDB and Elasticsearch initialization.
"""

import sys
import yaml
import getpass
from pathlib import Path

from mongodb_init import init_mongodb, create_admin_user
from es_init import init_elasticsearch


def load_config():
    """Load configuration from config.yml"""
    # Try to find config.yml in parent directory or current directory
    config_path = Path(__file__).parent.parent / "config.yml"
    if not config_path.exists():
        config_path = Path("config.yml")
        if not config_path.exists():
            raise FileNotFoundError("config.yml not found")
    
    with open(config_path, 'r') as f:
        return yaml.safe_load(f)


def main():
    print("=" * 60)
    print("mtv2 Backend Initialization Script")
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
    
    # Initialize MongoDB
    print("=" * 60)
    print("MongoDB Initialization")
    print("=" * 60)
    print()
    
    mongo_client = None
    try:
        mongo_client, db = init_mongodb(config)
    except Exception as e:
        print(f"✗ MongoDB initialization failed: {e}")
        sys.exit(1)
    
    # Initialize Elasticsearch
    print("=" * 60)
    print("Elasticsearch Initialization")
    print("=" * 60)
    print()
    
    es_client = None
    try:
        es_client = init_elasticsearch(config)
    except Exception as e:
        print(f"⚠ Elasticsearch initialization failed: {e}")
        print("⚠ Continuing with MongoDB initialization only...")
        print()
    
    # Create admin user (MongoDB only)
    print("=" * 60)
    print("Initial Admin User Setup")
    print("=" * 60)
    
    username = input("Enter admin username (or press Enter to skip): ").strip()
    if username:
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
        try:
            create_admin_user(db, config, username, password, email)
        except Exception as e:
            print(f"✗ Failed to create admin user: {e}")
    else:
        print("⚠ Skipping admin user creation")
    
    print()
    print("=" * 60)
    print("Initialization Summary")
    print("=" * 60)
    print("✓ MongoDB: Initialized")
    if es_client:
        print("✓ Elasticsearch: Initialized")
    else:
        print("⚠ Elasticsearch: Not initialized")
    print("=" * 60)
    
    # Cleanup
    if mongo_client:
        mongo_client.close()


if __name__ == "__main__":
    main()
