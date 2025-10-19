#!/usr/bin/env python3
"""
Main deployment script for xiaomei backend
Handles database initialization and config generation
"""

import sys
import os
import argparse
from pathlib import Path

# Add current directory to path for imports
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

from init_databases import init_databases
from generate_config import generate_backend_config

def main():
    parser = argparse.ArgumentParser(description='Deploy xiaomei backend')
    parser.add_argument('--production', action='store_true', 
                       help='Deploy for production environment')
    parser.add_argument('--skip-db-init', action='store_true',
                       help='Skip database initialization')
    parser.add_argument('--skip-config', action='store_true',
                       help='Skip config generation')
    
    args = parser.parse_args()
    
    # Determine environment
    environment = "production" if args.production else "development"
    
    print(f"🚀 Starting deployment for {environment} environment...")
    
    # Get project root directory (parent of deploy folder)
    project_root = Path(__file__).parent.parent
    config_file = project_root / "config.yml"
    
    if not config_file.exists():
        print("❌ Error: config.yml not found in project root")
        sys.exit(1)
    
    try:
        # Generate backend config
        if not args.skip_config:
            print("📝 Generating backend configuration...")
            generate_backend_config(str(config_file), environment)
            print("✅ Backend configuration generated")
        else:
            print("⏭️  Skipping config generation")
        
        # Initialize databases
        if not args.skip_db_init:
            print("🗄️  Initializing databases...")
            init_databases(environment)
            print("✅ Database initialization completed")
        else:
            print("⏭️  Skipping database initialization")
        
        print(f"🎉 Deployment completed successfully for {environment} environment!")
        print("\n📋 Next steps:")
        print("  1. Ensure all services (MongoDB, Elasticsearch, Redis) are running")
        print("  2. Run: cd backend && go run main.go")
        print("  3. Or build and run: cd backend && go build -o xiaomei-backend && ./xiaomei-backend")
        
    except Exception as e:
        print(f"❌ Deployment failed: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
