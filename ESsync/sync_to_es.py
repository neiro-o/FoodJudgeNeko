#!/usr/bin/env python3
"""
MongoDB to Elasticsearch Synchronization Script

This script reads documents from MongoDB's problems collection and syncs them
to Elasticsearch using the auto-detection converter.

Features:
- Auto-detects problem type based on processType
- Handles errors gracefully (skips failed documents)
- Bulk indexing for performance
- Progress tracking and statistics
- Detailed error logging
"""

import sys
import yaml
import logging
from pathlib import Path
from typing import Dict, List, Any, Tuple
from datetime import datetime

try:
    from pymongo import MongoClient
    from pymongo.errors import PyMongoError
except ImportError:
    print("Error: pymongo not installed. Run: pip install pymongo")
    sys.exit(1)

try:
    from elasticsearch import Elasticsearch
    from elasticsearch.helpers import bulk, BulkIndexError
    from elasticsearch.exceptions import TransportError, RequestError
except ImportError:
    print("Error: elasticsearch not installed. Run: pip install elasticsearch")
    sys.exit(1)

try:
    from converter import convert_bson_to_es, safe_get
except ImportError:
    print("Error: converter module not found. Make sure you're running from the ESsync directory")
    sys.exit(1)


# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler('sync_to_es.log'),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger(__name__)


class MongoToESSync:
    """Synchronize MongoDB problems collection to Elasticsearch."""
    
    def __init__(self, config_path: str = "../config.yml"):
        """
        Initialize the sync manager.
        
        Args:
            config_path: Path to config.yml file
        """
        self.config = self.load_config(config_path)
        self.mongo_client = None
        self.es_client = None
        self.stats = {
            'total': 0,
            'success': 0,
            'failed': 0,
            'skipped': 0,
            'by_type': {1: 0, 2: 0, 3: 0, 4: 0, 5: 0}
        }
        self.errors = []
    
    def load_config(self, config_path: str) -> Dict[str, Any]:
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
    
    def connect_mongodb(self) -> bool:
        """Connect to MongoDB."""
        try:
            mongo_config = self.config['mongodb']
            self.mongo_client = MongoClient(
                mongo_config['connection_string'],
                serverSelectionTimeoutMS=5000
            )
            # Test connection
            self.mongo_client.server_info()
            logger.info(f"✓ Connected to MongoDB: {mongo_config['connection_string']}")
            return True
        except PyMongoError as e:
            logger.error(f"✗ MongoDB connection failed: {e}")
            return False
    
    def connect_elasticsearch(self) -> bool:
        """Connect to Elasticsearch."""
        try:
            es_config = self.config['elasticsearch']
            self.es_client = Elasticsearch(
                es_config['hosts'],
                basic_auth=(es_config['username'], es_config['password']),
                verify_certs=False,  # Set to True in production with proper certs
                ssl_show_warn=False
            )
            # Test connection
            if self.es_client.ping():
                logger.info(f"✓ Connected to Elasticsearch: {es_config['hosts']}")
                return True
            else:
                logger.error("✗ Elasticsearch ping failed")
                return False
        except (TransportError, RequestError) as e:
            logger.error(f"✗ Elasticsearch connection failed: {e}")
            return False
    
    def fetch_all_documents(self) -> List[Dict[str, Any]]:
        """Fetch all documents from MongoDB problems collection."""
        try:
            db_name = self.config['mongodb']['database_name']
            collection_name = self.config['mongodb']['collections']['problems']
            
            db = self.mongo_client[db_name]
            collection = db[collection_name]
            
            # Count total documents
            total = collection.count_documents({})
            logger.info(f"📊 Found {total} documents in {db_name}.{collection_name}")
            
            # Fetch all documents
            documents = list(collection.find({}))
            logger.info(f"✓ Fetched {len(documents)} documents")
            
            return documents
        except PyMongoError as e:
            logger.error(f"✗ Failed to fetch documents: {e}")
            return []
    
    def convert_document(self, bson_doc: Dict[str, Any]) -> Tuple[Dict[str, Any], str]:
        """
        Convert a single BSON document to ES format.
        
        Args:
            bson_doc: MongoDB BSON document
            
        Returns:
            Tuple of (es_doc or None, error_message or None)
        """
        try:
            # Convert using auto-detection
            es_doc = convert_bson_to_es(bson_doc)
            
            # Track by type
            problem_type = es_doc.get('problem_type', 0)
            if problem_type in self.stats['by_type']:
                self.stats['by_type'][problem_type] += 1
            
            return es_doc, None
        except ValueError as e:
            # Type 5 not implemented
            error_msg = f"Type 5 not implemented: {e}"
            return None, error_msg
        except Exception as e:
            error_msg = f"Conversion error: {e}"
            return None, error_msg
    
    def prepare_bulk_actions(self, documents: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
        """
        Convert documents and prepare bulk actions for Elasticsearch.
        
        Args:
            documents: List of MongoDB documents
            
        Returns:
            List of Elasticsearch bulk actions
        """
        actions = []
        index_name = self.config['elasticsearch']['index_name']
        
        self.stats['total'] = len(documents)
        
        for i, bson_doc in enumerate(documents, 1):
            # Progress logging
            if i % 100 == 0 or i == len(documents):
                logger.info(f"Processing {i}/{len(documents)} documents...")
            
            # Convert document
            es_doc, error = self.convert_document(bson_doc)
            
            if error:
                # Log error and skip
                mongo_id = bson_doc.get('_id', 'unknown')
                problem_type = safe_get(bson_doc, "detail", "taskInfo", "processType", default="unknown")
                logger.warning(f"⚠️  Skipped document {mongo_id} (processType: {problem_type}): {error}")
                self.errors.append({'mongo_id': mongo_id, 'error': error, 'processType': problem_type})
                self.stats['skipped'] += 1
                continue
            
            # Prepare bulk action
            # Ensure mongo_id is a string (convert ObjectId to string)
            mongo_id = es_doc.get('mongo_id')
            if mongo_id is not None:
                mongo_id_str = str(mongo_id)
                # Also update in the document source to ensure consistency
                es_doc['mongo_id'] = mongo_id_str
            else:
                mongo_id_str = None
                # Log warning if mongo_id is missing
                logger.warning(f"⚠️  Document missing mongo_id, will use ES auto-generated ID")
            
            # Build action - only include _id if mongo_id_str is not None
            # If None, ES will auto-generate the ID
            action = {
                '_index': index_name,
                '_source': es_doc
            }
            if mongo_id_str is not None:
                action['_id'] = mongo_id_str
            
            actions.append(action)
        
        logger.info(f"✓ Prepared {len(actions)} documents for indexing")
        return actions
    
    def bulk_index(self, actions: List[Dict[str, Any]]) -> bool:
        """
        Bulk index documents to Elasticsearch.
        
        Args:
            actions: List of Elasticsearch bulk actions
            
        Returns:
            True if successful, False otherwise
        """
        if not actions:
            logger.warning("⚠️  No documents to index")
            return True
        
        try:
            # Perform bulk indexing
            success, failed = bulk(
                self.es_client,
                actions,
                raise_on_error=False,
                raise_on_exception=False
            )
            
            self.stats['success'] = success
            self.stats['failed'] = len(failed) if isinstance(failed, list) else 0
            
            logger.info(f"✓ Bulk indexing complete: {success} succeeded, {self.stats['failed']} failed")
            
            # Log failed documents with more details
            if failed and isinstance(failed, list):
                logger.error(f"✗ {len(failed)} document(s) failed to index")
                for i, item in enumerate(failed[:20], 1):  # Show first 20 failures
                    error_info = item.get('index', {}) if isinstance(item, dict) else item
                    error_data = error_info.get('error', {}) if isinstance(error_info, dict) else {}
                    error_reason = error_data.get('reason', 'Unknown error') if isinstance(error_data, dict) else str(error_data)
                    doc_id = error_info.get('_id', 'unknown') if isinstance(error_info, dict) else 'unknown'
                    logger.error(f"  [{i}] ID: {doc_id}, Error: {error_reason}")
            
            return True
            
        except BulkIndexError as e:
            logger.error(f"✗ Bulk indexing error: {e}")
            return False
        except (TransportError, RequestError) as e:
            logger.error(f"✗ Elasticsearch error: {e}")
            return False
    
    def print_statistics(self):
        """Print synchronization statistics."""
        print("\n" + "=" * 80)
        print("SYNCHRONIZATION STATISTICS")
        print("=" * 80)
        print(f"Total documents:        {self.stats['total']}")
        print(f"Successfully indexed:   {self.stats['success']}")
        print(f"Failed to index:        {self.stats['failed']}")
        print(f"Skipped (errors):       {self.stats['skipped']}")
        print()
        print("By Type:")
        for ptype, count in sorted(self.stats['by_type'].items()):
            if count > 0:
                print(f"  Type {ptype}: {count} documents")
        print()
        
        if self.errors:
            print(f"⚠️  {len(self.errors)} documents skipped due to errors")
            print()
            print("First 10 errors:")
            for error in self.errors[:10]:
                print(f"  - {error['mongo_id']}: {error['error']}")
            print()
        
        print("=" * 80)
    
    def sync(self) -> bool:
        """
        Main synchronization process.
        
        Returns:
            True if successful, False otherwise
        """
        start_time = datetime.now()
        logger.info("=" * 80)
        logger.info("Starting MongoDB to Elasticsearch Synchronization")
        logger.info("=" * 80)
        
        # Step 1: Connect to MongoDB
        logger.info("\n[1/5] Connecting to MongoDB...")
        if not self.connect_mongodb():
            return False
        
        # Step 2: Connect to Elasticsearch
        logger.info("\n[2/5] Connecting to Elasticsearch...")
        if not self.connect_elasticsearch():
            return False
        
        # Step 3: Fetch documents
        logger.info("\n[3/5] Fetching documents from MongoDB...")
        documents = self.fetch_all_documents()
        if not documents:
            logger.warning("⚠️  No documents found")
            return False
        
        # Step 4: Convert documents
        logger.info("\n[4/5] Converting documents...")
        actions = self.prepare_bulk_actions(documents)
        
        # Step 5: Bulk index to Elasticsearch
        logger.info("\n[5/5] Indexing to Elasticsearch...")
        success = self.bulk_index(actions)
        
        # Calculate duration
        duration = datetime.now() - start_time
        
        # Print statistics
        self.print_statistics()
        
        logger.info(f"\n✓ Synchronization completed in {duration}")
        logger.info("=" * 80)
        
        return success
    
    def close(self):
        """Close database connections."""
        if self.mongo_client:
            self.mongo_client.close()
            logger.info("✓ MongoDB connection closed")
        
        if self.es_client:
            self.es_client.close()
            logger.info("✓ Elasticsearch connection closed")


def main():
    """Main entry point."""
    print("""
╔════════════════════════════════════════════════════════════════════════════╗
║           MongoDB to Elasticsearch Synchronization Script                  ║
║                        ESsync v1.5.0                                        ║
╚════════════════════════════════════════════════════════════════════════════╝
    """)
    
    # Initialize sync manager
    sync_manager = MongoToESSync()
    
    try:
        # Run synchronization
        success = sync_manager.sync()
        
        if success:
            print("\n✅ Synchronization completed successfully!")
            sys.exit(0)
        else:
            print("\n❌ Synchronization failed")
            sys.exit(1)
    
    except KeyboardInterrupt:
        print("\n\n⚠️  Interrupted by user")
        logger.warning("Synchronization interrupted by user")
        sys.exit(1)
    
    except Exception as e:
        print(f"\n❌ Unexpected error: {e}")
        logger.error(f"Unexpected error: {e}", exc_info=True)
        sys.exit(1)
    
    finally:
        # Clean up
        sync_manager.close()


if __name__ == "__main__":
    main()
