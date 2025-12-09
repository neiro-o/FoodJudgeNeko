#!/usr/bin/env python3
"""
Clear Elasticsearch Index Data

This script clears all documents from the Elasticsearch problems index
without deleting the index itself (preserves index structure and mappings).

Features:
- Safe confirmation prompt before deletion
- Shows document count before and after
- Preserves index structure (mappings, settings)
- Detailed logging

Usage:
    python clear_es.py              # Clear with confirmation prompt
    python clear_es.py --yes        # Skip confirmation (for automation)
    python clear_es.py --dry-run    # Show what would be deleted without deleting
"""

import sys
import yaml
import logging
import argparse
from pathlib import Path
from typing import Dict, Any

try:
    from elasticsearch import Elasticsearch
    from elasticsearch.exceptions import TransportError, RequestError, NotFoundError
except ImportError:
    print("Error: elasticsearch not installed. Run: pip install elasticsearch")
    sys.exit(1)


# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler('clear_es.log'),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger(__name__)


def load_config(config_path: str = "../config.yml") -> Dict[str, Any]:
    """Load configuration from YAML file."""
    try:
        config_file = Path(__file__).parent / config_path
        with open(config_file, 'r', encoding='utf-8') as f:
            config = yaml.safe_load(f)
        logger.info(f"✓ Loaded configuration from {config_file}")
        return config
    except Exception as e:
        logger.error(f"✗ Failed to load config: {e}")
        sys.exit(1)


def connect_elasticsearch(config: Dict[str, Any]) -> Elasticsearch:
    """Connect to Elasticsearch."""
    try:
        es_config = config['elasticsearch']
        es_client = Elasticsearch(
            es_config['hosts'],
            basic_auth=(es_config['username'], es_config['password']),
            verify_certs=False,  # Set to True in production with proper certs
            ssl_show_warn=False
        )
        # Test connection
        if es_client.ping():
            logger.info(f"✓ Connected to Elasticsearch: {es_config['hosts']}")
            return es_client
        else:
            logger.error("✗ Elasticsearch ping failed")
            sys.exit(1)
    except (TransportError, RequestError) as e:
        logger.error(f"✗ Elasticsearch connection failed: {e}")
        sys.exit(1)


def get_document_count(es: Elasticsearch, index_name: str) -> int:
    """Get the total number of documents in the index."""
    try:
        if not es.indices.exists(index=index_name):
            return 0
        result = es.count(index=index_name)
        return result['count']
    except NotFoundError:
        return 0
    except (TransportError, RequestError) as e:
        logger.error(f"✗ Failed to count documents: {e}")
        return -1


def clear_index(es: Elasticsearch, index_name: str, dry_run: bool = False) -> bool:
    """
    Clear all documents from the Elasticsearch index.
    
    Args:
        es: Elasticsearch client
        index_name: Name of the index to clear
        dry_run: If True, only show what would be deleted without actually deleting
        
    Returns:
        True if successful, False otherwise
    """
    # Check if index exists
    if not es.indices.exists(index=index_name):
        logger.warning(f"⚠️  Index '{index_name}' does not exist")
        print(f"ℹ️  Index '{index_name}' does not exist (nothing to clear)")
        return False
    
    # Get document count
    doc_count = get_document_count(es, index_name)
    
    if doc_count == 0:
        logger.info(f"ℹ️  Index '{index_name}' is already empty")
        print(f"ℹ️  Index '{index_name}' is already empty (no documents to delete)")
        return True
    
    if doc_count < 0:
        logger.error("✗ Failed to get document count")
        print("✗ Failed to get document count")
        return False
    
    if dry_run:
        print(f"\n🔍 DRY RUN: Would delete {doc_count:,} document(s) from index '{index_name}'")
        print("   (No documents were actually deleted)")
        return True
    
    # Delete all documents using delete_by_query
    try:
        logger.info(f"Starting deletion of {doc_count:,} documents from '{index_name}'")
        print(f"\n🗑️  Deleting {doc_count:,} document(s) from index '{index_name}'...")
        
        # Use delete_by_query to delete all documents matching a match_all query
        result = es.delete_by_query(
            index=index_name,
            body={
                "query": {
                    "match_all": {}
                }
            },
            wait_for_completion=True,
            refresh=True
        )
        
        deleted_count = result.get('deleted', 0)
        logger.info(f"✓ Deleted {deleted_count:,} documents")
        print(f"✓ Successfully deleted {deleted_count:,} document(s)")
        
        # Verify deletion
        remaining = get_document_count(es, index_name)
        if remaining == 0:
            print(f"✓ Index '{index_name}' is now empty")
            logger.info(f"✓ Index '{index_name}' cleared successfully")
            return True
        else:
            print(f"⚠️  Warning: {remaining:,} document(s) still remain in the index")
            logger.warning(f"⚠️  {remaining:,} documents still remain after deletion")
            return False
            
    except (TransportError, RequestError) as e:
        logger.error(f"✗ Failed to delete documents: {e}")
        print(f"✗ Failed to delete documents: {e}")
        return False


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description="Clear all documents from Elasticsearch index",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python clear_es.py              # Clear with confirmation prompt
  python clear_es.py --yes        # Skip confirmation
  python clear_es.py --dry-run    # Show what would be deleted
        """
    )
    parser.add_argument(
        '--yes', '-y',
        action='store_true',
        help='Skip confirmation prompt (automated use)'
    )
    parser.add_argument(
        '--dry-run',
        action='store_true',
        help='Show what would be deleted without actually deleting'
    )
    parser.add_argument(
        '--config',
        type=str,
        default='../config.yml',
        help='Path to config.yml (default: ../config.yml)'
    )
    
    args = parser.parse_args()
    
    print("""
╔════════════════════════════════════════════════════════════════════════════╗
║                  Elasticsearch Index Data Clearer                          ║
║                         ESsync v1.0                                        ║
╚════════════════════════════════════════════════════════════════════════════╝
    """)
    
    # Load config
    config = load_config(args.config)
    index_name = config['elasticsearch']['index_name']
    
    # Connect to Elasticsearch
    logger.info("Connecting to Elasticsearch...")
    es = connect_elasticsearch(config)
    
    # Get current document count
    doc_count = get_document_count(es, index_name)
    
    if doc_count < 0:
        print("✗ Failed to connect to index")
        sys.exit(1)
    
    # Show current status
    print("\n" + "=" * 80)
    print("CURRENT INDEX STATUS")
    print("=" * 80)
    print(f"Index name: {index_name}")
    print(f"Document count: {doc_count:,}")
    print("=" * 80)
    
    if doc_count == 0:
        print("\n✓ Index is already empty. Nothing to do.")
        sys.exit(0)
    
    # Confirmation prompt (unless --yes or --dry-run)
    if not args.yes and not args.dry_run:
        print(f"\n⚠️  WARNING: This will delete ALL {doc_count:,} document(s) from index '{index_name}'")
        print("   The index structure will be preserved, but all data will be lost.")
        print()
        response = input("Are you sure you want to continue? (yes/no): ").strip().lower()
        if response not in ['yes', 'y']:
            print("\n❌ Operation cancelled by user")
            sys.exit(0)
        print()
    
    # Clear the index
    print("\n" + "=" * 80)
    if args.dry_run:
        print("DRY RUN MODE")
    else:
        print("CLEARING INDEX")
    print("=" * 80)
    
    success = clear_index(es, index_name, dry_run=args.dry_run)
    
    # Final status
    print("\n" + "=" * 80)
    if success:
        if args.dry_run:
            print("✅ Dry run completed successfully")
        else:
            print("✅ Index cleared successfully!")
            print("\nNext steps:")
            print("  1. Run sync script to repopulate index:")
            print("     python sync_to_es.py")
            print("  2. Verify document count:")
            print("     python count_docs.py")
    else:
        print("❌ Failed to clear index")
        sys.exit(1)
    print("=" * 80 + "\n")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\n\n⚠️  Interrupted by user")
        logger.warning("Operation interrupted by user")
        sys.exit(1)
    except Exception as e:
        print(f"\n❌ Unexpected error: {e}")
        logger.error(f"Unexpected error: {e}", exc_info=True)
        sys.exit(1)
