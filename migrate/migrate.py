#!/usr/bin/env python3
"""
MongoDB Migration Script
Migrates data from source MongoDB to target MongoDB with SSH tunnel support.
"""

import sys
import os
import yaml
import argparse
from typing import Optional, Dict, Any
from pathlib import Path
from pymongo import MongoClient
from pymongo.errors import BulkWriteError
from sshtunnel import SSHTunnelForwarder
import random


def load_config(config_path: str) -> Dict[str, Any]:
    """Load YAML configuration file"""
    try:
        with open(config_path, 'r') as f:
            return yaml.safe_load(f)
    except FileNotFoundError:
        print(f"Error: Config file not found: {config_path}")
        sys.exit(1)
    except yaml.YAMLError as e:
        print(f"Error: Failed to parse config file: {e}")
        sys.exit(1)


def create_ssh_tunnel(ssh_config: Dict[str, Any]) -> Optional[SSHTunnelForwarder]:
    """Create SSH tunnel for MongoDB connection"""
    if not ssh_config.get('enabled', False):
        return None
    
    print(f"Setting up SSH tunnel to {ssh_config['host']}:{ssh_config.get('port', 22)}...")
    
    # Determine authentication method
    ssh_kwargs = {
        'ssh_address_or_host': (ssh_config['host'], ssh_config.get('port', 22)),
        'ssh_username': ssh_config['username'],
        'remote_bind_address': ('127.0.0.1', 27017),
    }
    
    # Add local port if specified
    if ssh_config.get('local_port'):
        ssh_kwargs['local_bind_address'] = ('127.0.0.1', ssh_config['local_port'])
    
    # Authentication: prefer key_file, then key_string, then password
    if ssh_config.get('key_file'):
        # Expand user home directory and resolve path
        key_file_path = os.path.expanduser(ssh_config['key_file'])
        key_file_path = os.path.expandvars(key_file_path)
        key_file_path = os.path.abspath(key_file_path)
        if not os.path.exists(key_file_path):
            print(f"✗ SSH key file not found: {key_file_path}")
            sys.exit(1)
        ssh_kwargs['ssh_pkey'] = key_file_path
        print(f"  Using SSH key file: {key_file_path}")
    elif ssh_config.get('key_string'):
        import io
        ssh_kwargs['ssh_pkey'] = io.StringIO(ssh_config['key_string'])
        print("  Using SSH key string")
    elif ssh_config.get('password'):
        ssh_kwargs['ssh_password'] = ssh_config['password']
        print("  Using SSH password authentication")
    else:
        print("  Warning: No authentication method specified, trying default SSH key")
    
    try:
        tunnel = SSHTunnelForwarder(**ssh_kwargs)
        tunnel.start()
        local_port = tunnel.local_bind_port
        print(f"✓ SSH tunnel established on localhost:{local_port}")
        return tunnel
    except Exception as e:
        print(f"✗ Failed to create SSH tunnel: {e}")
        sys.exit(1)


def connect_to_mongodb(connection_string: str, database_name: str, db_label: str = "MongoDB") -> tuple:
    """
    Connect to MongoDB and return client and database.
    Uses the same connection pattern as setup scripts.
    """
    print(f"Connecting to {db_label}...")
    print(f"  Connection string: {connection_string}")
    print(f"  Database: {database_name}")
    
    try:
        # Use same connection parameters as setup scripts
        client = MongoClient(connection_string, serverSelectionTimeoutMS=10000)
        # Test connection
        client.server_info()
        db = client[database_name]
        print(f"✓ Connected to {db_label} successfully")
        return client, db
    except Exception as e:
        print(f"✗ Failed to connect to {db_label}: {e}")
        print(f"  Error type: {type(e).__name__}")
        sys.exit(1)


def get_collection_count(collection) -> int:
    """Get total count of documents in collection"""
    return collection.count_documents({})


def clear_target_database(target_db, target_collections: Dict[str, str], require_confirmation: bool = True) -> None:
    """
    Clear problems and notes collections in the target database.
    Requires user confirmation before proceeding.
    """
    print("\n" + "="*60)
    print("Clear Target MongoDB")
    print("="*60)
    
    # Only clear problems and notes collections
    collections_to_clear = ['problems', 'notes']
    collection_info = []
    for col_key in collections_to_clear:
        if col_key in target_collections:
            col_name = target_collections[col_key]
            collection = target_db[col_name]
            count = get_collection_count(collection)
            collection_info.append((col_name, count))
        else:
            print(f"⚠ Warning: Collection '{col_key}' not found in target_collections config")
    
    # Display what will be cleared
    print("\nCollections to be cleared:")
    total_docs = 0
    for col_name, count in collection_info:
        print(f"  - {col_name}: {count} documents")
        total_docs += count
    
    print(f"\nTotal documents to be deleted: {total_docs}")
    
    if total_docs == 0:
        print("⚠ No documents to clear. Target database is already empty.")
        return
    
    # Require confirmation
    if require_confirmation:
        print("\n⚠ WARNING: This will permanently delete all data in the 'problems' and 'notes' collections!")
        print(f"  Database: {target_db.name}")
        print(f"  Collections: {', '.join([col for col, _ in collection_info])}")
        
        response = input("\nType 'yes' to confirm clearing the 'problems' and 'notes' collections: ").strip().lower()
        
        if response != 'yes':
            print("✗ Operation cancelled. No data was cleared.")
            return
    
    # Clear collections
    print("\nClearing collections...")
    for col_name, count in collection_info:
        if count > 0:
            collection = target_db[col_name]
            result = collection.delete_many({})
            print(f"✓ Cleared {col_name}: {result.deleted_count} documents deleted")
        else:
            print(f"  {col_name}: already empty")
    
    print("\n✓ Target database cleared successfully!")


def export_collection(
    source_collection,
    target_collection,
    collection_name: str,
    mode: str = "full",
    percentile: float = 100.0,
    batch_size: int = 1000
) -> int:
    """Export documents from source to target collection"""
    print(f"\n{'='*60}")
    print(f"Migrating collection: {collection_name}")
    print(f"{'='*60}")
    
    # Get total count
    total_count = get_collection_count(source_collection)
    print(f"Total documents in source: {total_count}")
    
    if total_count == 0:
        print("⚠ No documents to migrate")
        return 0
    
    # Determine how many documents to export
    if mode == "percentile":
        export_count = int(total_count * (percentile / 100.0))
        print(f"Exporting {percentile}% ({export_count} documents)")
    else:
        export_count = total_count
        print(f"Exporting all {export_count} documents")
    
    # Get documents
    if mode == "percentile" and percentile < 100:
        # For percentile, we'll use random sampling
        # First, get all document IDs
        print("Sampling documents...")
        all_ids = [doc['_id'] for doc in source_collection.find({}, {'_id': 1})]
        sample_size = min(export_count, len(all_ids))
        sampled_ids = random.sample(all_ids, sample_size)
        
        # Fetch sampled documents
        cursor = source_collection.find({'_id': {'$in': sampled_ids}})
    else:
        # Full export
        cursor = source_collection.find({})
    
    # Batch insert
    migrated_count = 0
    duplicate_count = 0
    duplicate_ids = []
    batch = []
    
    print(f"\nMigrating documents (batch size: {batch_size})...")
    for doc in cursor:
        batch.append(doc)
        
        if len(batch) >= batch_size:
            try:
                result = target_collection.insert_many(batch, ordered=False)
                migrated_count += len(result.inserted_ids)
                print(f"  Migrated {migrated_count}/{export_count} documents...", end='\r')
                batch = []
            except BulkWriteError as e:
                # Handle duplicates or other errors
                write_errors = e.details.get('writeErrors', [])
                num_inserted = e.details.get('nInserted', 0)
                migrated_count += num_inserted
                
                # Track duplicates (error code 11000 is duplicate key)
                for error in write_errors:
                    if error.get('code') == 11000:
                        duplicate_count += 1
                        # Extract the duplicate _id from the error message or document
                        if error.get('index') is not None and error['index'] < len(batch):
                            dup_id = batch[error['index']].get('_id', 'unknown')
                            duplicate_ids.append(str(dup_id))
                
                print(f"  Migrated {migrated_count}/{export_count} documents ({duplicate_count} duplicates skipped)...", end='\r')
                batch = []
            except Exception as e:
                print(f"\n✗ Failed to insert batch: {e}")
                raise
    
    # Insert remaining documents
    if batch:
        try:
            result = target_collection.insert_many(batch, ordered=False)
            migrated_count += len(result.inserted_ids)
        except BulkWriteError as e:
            write_errors = e.details.get('writeErrors', [])
            num_inserted = e.details.get('nInserted', 0)
            migrated_count += num_inserted
            
            # Track duplicates
            for error in write_errors:
                if error.get('code') == 11000:
                    duplicate_count += 1
                    if error.get('index') is not None and error['index'] < len(batch):
                        dup_id = batch[error['index']].get('_id', 'unknown')
                        duplicate_ids.append(str(dup_id))
        except Exception as e:
            print(f"\n✗ Failed to insert remaining batch: {e}")
            raise
    
    print(f"\n✓ Migration complete: {migrated_count} documents migrated")
    
    if duplicate_count > 0:
        print(f"⚠ Skipped {duplicate_count} duplicate documents")
        if duplicate_ids and len(duplicate_ids) <= 20:
            print(f"  Duplicate IDs: {', '.join(duplicate_ids[:20])}")
        elif duplicate_ids:
            print(f"  First 20 duplicate IDs: {', '.join(duplicate_ids[:20])}")
            print(f"  ... and {len(duplicate_ids) - 20} more")
    
    return migrated_count


def main():
    parser = argparse.ArgumentParser(description='Migrate MongoDB data from source to target')
    parser.add_argument(
        '--config',
        type=str,
        default='migrate_config.yml',
        help='Path to migration config file (default: migrate_config.yml)'
    )
    parser.add_argument(
        '--target-config',
        type=str,
        default='../config.yml',
        help='Path to target MongoDB config file (default: ../config.yml)'
    )
    parser.add_argument(
        '--dry-run',
        action='store_true',
        help='Perform a dry run without actually migrating data'
    )
    parser.add_argument(
        '--clear-target',
        action='store_true',
        help='Clear data from problems and notes collections in target MongoDB (requires confirmation)'
    )
    
    args = parser.parse_args()
    
    # Resolve config paths
    script_dir = Path(__file__).parent
    migrate_config_path = script_dir / args.config
    target_config_path = script_dir / args.target_config
    
    # Load configurations
    print(f"Loading migration configuration from: {migrate_config_path}")
    migrate_config = load_config(str(migrate_config_path))
    
    print(f"Loading target MongoDB configuration from: {target_config_path}")
    target_config = load_config(str(target_config_path))
    
    if args.dry_run:
        print("\n⚠ DRY RUN MODE - No data will be migrated\n")
    
    # Get target connection info first (needed for clear mode)
    target_connection_string = target_config['mongodb']['connection_string']
    target_database_name = target_config['mongodb']['database_name']
    target_collections = target_config['mongodb']['collections']
    
    # Handle clear-target mode
    if args.clear_target:
        print("\n⚠ CLEAR TARGET MODE - Will clear target MongoDB collections\n")
        
        # Connect to target MongoDB only
        print("\n" + "="*60)
        print("Connecting to Target MongoDB")
        print("="*60)
        target_client, target_db = connect_to_mongodb(
            target_connection_string,
            target_database_name,
            db_label="Target MongoDB"
        )
        
        try:
            clear_target_database(target_db, target_collections, require_confirmation=True)
            target_client.close()
            print("\n✓ Clear operation completed successfully!")
            return
        except KeyboardInterrupt:
            print("\n\n⚠ Operation interrupted by user")
            target_client.close()
            sys.exit(1)
        except Exception as e:
            print(f"\n✗ Clear operation failed: {e}")
            import traceback
            traceback.print_exc()
            target_client.close()
            sys.exit(1)
    
    # Setup SSH tunnel if needed (for migration mode)
    tunnel = None
    if migrate_config['source'].get('ssh', {}).get('enabled', False):
        tunnel = create_ssh_tunnel(migrate_config['source']['ssh'])
    
    # Get connection strings
    source_connection_string = migrate_config['source']['connection_string']
    source_database_name = migrate_config['source']['database_name']
    
    try:
        # Connect to source MongoDB (may use SSH tunnel if enabled)
        print("\n" + "="*60)
        print("Connecting to Source MongoDB")
        print("="*60)
        source_client, source_db = connect_to_mongodb(
            source_connection_string,
            source_database_name,
            db_label="Source MongoDB"
        )
        
        # Connect to target MongoDB - same method as setup scripts
        print("\n" + "="*60)
        print("Connecting to Target MongoDB")
        print("="*60)
        target_client, target_db = connect_to_mongodb(
            target_connection_string,
            target_database_name,
            db_label="Target MongoDB"
        )
        
        # Get migration settings
        migration_config = migrate_config.get('migration', {})
        mode = migration_config.get('mode', 'full')
        percentile = float(migration_config.get('percentile', 100))
        batch_size = migration_config.get('batch_size', 1000)
        
        if args.dry_run:
            print("\n" + "="*60)
            print("DRY RUN - Would migrate:")
            print("="*60)
            
            # Show what would be migrated
            source_collections = migrate_config['source']['collections']
            for source_col, target_col in [
                ('meituan', 'problems'),
                ('manual', 'notes')
            ]:
                source_col_name = source_collections.get(source_col)
                if source_col_name:
                    source_collection = source_db[source_col_name]
                    count = get_collection_count(source_collection)
                    export_count = count if mode == 'full' else int(count * (percentile / 100.0))
                    print(f"  {source_col_name} ({count} docs) -> {target_col} ({export_count} docs)")
            
            print("\n⚠ No data was actually migrated (dry run mode)")
        else:
            # Get collection names from configs
            source_collections = migrate_config['source']['collections']
            
            # Migrate meituan -> problems
            source_meituan = source_collections.get('meituan', 'meituan')
            target_problems = target_collections.get('problems', 'problems')
            
            export_collection(
                source_db[source_meituan],
                target_db[target_problems],
                f"{source_meituan} -> {target_problems}",
                mode=mode,
                percentile=percentile,
                batch_size=batch_size
            )
            
            # Migrate manual -> notes
            source_manual = source_collections.get('manual', 'manual')
            target_notes = target_collections.get('notes', 'notes')
            
            export_collection(
                source_db[source_manual],
                target_db[target_notes],
                f"{source_manual} -> {target_notes}",
                mode=mode,
                percentile=percentile,
                batch_size=batch_size
            )
            
            print("\n" + "="*60)
            print("✓ Migration completed successfully!")
            print("="*60)
        
        # Close connections
        source_client.close()
        target_client.close()
        
    except KeyboardInterrupt:
        print("\n\n⚠ Migration interrupted by user")
        sys.exit(1)
    except Exception as e:
        print(f"\n✗ Migration failed: {e}")
        print("\nDebugging information:")
        print("  - Source connection:", source_connection_string)
        print("  - Target connection:", target_connection_string)
        import traceback
        traceback.print_exc()
        sys.exit(1)
    finally:
        # Close SSH tunnel
        if tunnel:
            print("\nClosing SSH tunnel...")
            tunnel.stop()
            print("✓ SSH tunnel closed")


if __name__ == '__main__':
    main()
