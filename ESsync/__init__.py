"""
ESsync - MongoDB BSON to Elasticsearch Document Converter

This package provides functions to convert Meituan evaluation data from MongoDB 
BSON format to Elasticsearch document format.

Version 1.6.0 Changes:
- Added production sync script (sync_to_es.py)
- MongoDB to Elasticsearch bulk synchronization
- Graceful error handling and detailed statistics
- Complete sync documentation

Version 1.5.0 Changes:
- Added auto-detection function (convert_bson_to_es)
- Automatically routes to correct converter based on processType
- Type 5 (IPR) reserved for future implementation
- Supports all processType values across Types 1-4

Version 1.4.0 Changes:
- Added Type 4 converter (Dine-in refunds/堂食退款)
- Type 4 user_review shows "Refund for {price}"
- Type 4 timestamp is first appeal timestamp
- Type 4 has empty replies list
- Type 4 order_info with snapshotOrder data
- Appeals sorted by timestamp

Version 1.3.0 Changes:
- Added Type 3 converter (Waimai refunds/外卖退款)
- Type 3 has no user_review, review_pics, or timestamp
- Type 3 has empty replies list
- Type 3 orders from refundProductionList
- Appeals sorted by timestamp

Version 1.2.0 Changes:
- Added Type 2 converter (Dine-in/堂食 reviews)
- Type 2 extracts numbers from replyName for "others" users
- Type 2 has order_info with shop details instead of orders/order_detail

Version 1.1.0 Changes:
- All timestamps are now integers (not floats)
- Replies are automatically sorted by timestamp
"""

from .converter import (
    convert_bson_to_es,
    convert_bson_to_es_type1,
    convert_bson_to_es_type2,
    convert_bson_to_es_type3,
    convert_bson_to_es_type4,
    safe_get
)

__version__ = "1.6.0"
__all__ = [
    "convert_bson_to_es",
    "convert_bson_to_es_type1",
    "convert_bson_to_es_type2",
    "convert_bson_to_es_type3",
    "convert_bson_to_es_type4",
    "safe_get"
]
