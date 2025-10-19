#!/usr/bin/env python3
"""
Configuration generation script
Reads root config.yml and generates backend/config.yml with session secret
"""

import yaml
import secrets
import os
import sys
from pathlib import Path

def generate_session_secret():
    """Generate a random 32-character hex session secret"""
    return secrets.token_hex(16)

def read_root_config(config_path):
    """Read the root config.yml file"""
    try:
        with open(config_path, 'r', encoding='utf-8') as file:
            config = yaml.safe_load(file)
        return config
    except Exception as e:
        raise Exception(f"Failed to read config file {config_path}: {e}")

def generate_backend_config(config_path, environment="development"):
    """Generate backend/config.yml from root config.yml"""
    
    # Read root configuration
    root_config = read_root_config(config_path)
    
    # Generate session secret
    session_secret = generate_session_secret()
    print(f"🔐 Generated session secret: {session_secret}")
    
    # Determine secure cookie setting based on environment
    secure_cookie = environment == "production"
    
    # Create backend configuration
    backend_config = {
        "server": root_config.get("server", {"port": "8080"}),
        "mongodb": root_config.get("mongodb", {
            "uri": "mongodb://localhost:27017",
            "database": "mt_backend",
            "collections": {
                "users": "users",
                "invite_codes": "invite_codes",
                "raw_problems": "raw_problems"
            }
        }),
        "redis": root_config.get("redis", {
            "host": "localhost",
            "port": "6379",
            "password": "",
            "db": 0,
            "keys": {
                "upload_queue": "upload_queue"
            }
        }),
        "elasticsearch": root_config.get("elasticsearch", {
            "addresses": ["http://localhost:9200"],
            "index": "problems"
        }),
        "session": {
            "secret": session_secret,
            "max_age": 604800,  # 7 days in seconds
            "secure_cookie": secure_cookie,
            "http_only": True,
            "same_site": "Lax"
        }
    }
    
    # Ensure backend directory exists
    backend_dir = Path(config_path).parent / "backend"
    backend_dir.mkdir(exist_ok=True)
    
    # Write backend config
    backend_config_path = backend_dir / "config.yml"
    try:
        with open(backend_config_path, 'w', encoding='utf-8') as file:
            yaml.dump(backend_config, file, default_flow_style=False, sort_keys=False)
        
        print(f"📝 Generated backend configuration: {backend_config_path}")
        print(f"🌍 Environment: {environment}")
        print(f"🔒 Secure cookies: {'enabled' if secure_cookie else 'disabled'}")
        
    except Exception as e:
        raise Exception(f"Failed to write backend config: {e}")

if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser(description='Generate backend configuration')
    parser.add_argument('config_path', help='Path to root config.yml')
    parser.add_argument('--environment', default='development', 
                       help='Environment (development/production)')
    args = parser.parse_args()
    
    try:
        generate_backend_config(args.config_path, args.environment)
    except Exception as e:
        print(f"❌ Config generation failed: {e}")
        sys.exit(1)
