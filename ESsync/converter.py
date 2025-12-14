"""
MongoDB BSON to Elasticsearch Document Converter
Converts Meituan evaluation data (Type 1: Waimai/Takeaway) from MongoDB format to ES format
"""

from typing import Dict, Any, List, Optional, Union


def safe_get(data: Dict[str, Any], *keys, default=None) -> Any:
    """
    Safely navigate nested dictionary keys without raising KeyError.
    
    Args:
        data: The dictionary to navigate
        *keys: Sequence of keys to traverse
        default: Default value to return if key doesn't exist
        
    Returns:
        The value at the nested key path, or default if not found
    """
    result = data
    for key in keys:
        if isinstance(result, dict):
            result = result.get(key)
            if result is None:
                return default
        else:
            return default
    return result if result is not None else default


def convert_reply_role(reply_type: Optional[str], index: int = 0) -> str:
    """
    Convert replyType to standardized role (for Type 1).
    
    Args:
        reply_type: Original reply type from BSON
        index: Index for "others" type users
        
    Returns:
        Standardized role string
    """
    if not reply_type:
        return f"others_{index}"
    
    if reply_type == "MERCHANT":
        return "merchant"
    elif reply_type == "CUSTOMER":
        return "user"
    else:
        return f"others_{index}"


def convert_reply_role_type2(reply_type: Optional[str], reply_name: Optional[str]) -> str:
    """
    Convert replyType to standardized role (for Type 2).
    For Type 2, extract number from replyName for "others" users.
    
    Args:
        reply_type: Original reply type from BSON
        reply_name: Reply name like "路人1", "路人2", "商户", etc.
        
    Returns:
        Standardized role string
    """
    if reply_type == "MERCHANT":
        return "merchant"
    elif reply_type == "CUSTOMER":
        return "user"
    else:
        # Extract number from reply_name (e.g., "路人1" -> "others_1")
        if reply_name:
            import re
            match = re.search(r'(\d+)', reply_name)
            if match:
                num = match.group(1)
                return f"others_{num}"
        return "others_0"


def convert_delivery_type(delivery_type_cn: Optional[str]) -> str:
    """
    Convert Chinese delivery type to English code.
    
    Args:
        delivery_type_cn: Chinese delivery type string
        
    Returns:
        English delivery type code
    """
    if not delivery_type_cn:
        return "others"
    
    mapping = {
        "美团配送": "meituan",
        "商家配送": "merchant",
        "顾客自取": "user",
        "用户自取": "user"
    }
    
    return mapping.get(delivery_type_cn, "others")


def extract_pic_urls(picture_list: List[Any], video_list: List[Any]) -> List[str]:
    """
    Extract URL strings from picture and video objects.
    
    Picture objects may have: url, picUrl, coverUrl
    Video objects may have: videoUrl, m3u8Url, coverUrl
    
    Args:
        picture_list: List of picture objects
        video_list: List of video objects
        
    Returns:
        List of URL strings
    """
    urls = []
    
    # Extract URLs from picture objects
    if isinstance(picture_list, list):
        for pic_obj in picture_list:
            if isinstance(pic_obj, dict):
                # Try common URL fields
                url = (pic_obj.get('url') or 
                       pic_obj.get('picUrl') or 
                       pic_obj.get('coverUrl') or
                       pic_obj.get('imageUrl'))
                if url:
                    urls.append(str(url))
            elif isinstance(pic_obj, str):
                # Already a URL string
                urls.append(pic_obj)
    
    # Extract URLs from video objects
    if isinstance(video_list, list):
        for video_obj in video_list:
            if isinstance(video_obj, dict):
                # Try common URL fields (prefer m3u8Url for videos, fallback to videoUrl or coverUrl)
                url = (video_obj.get('m3u8Url') or 
                       video_obj.get('videoUrl') or 
                       video_obj.get('coverUrl'))
                if url:
                    urls.append(str(url))
            elif isinstance(video_obj, str):
                # Already a URL string
                urls.append(video_obj)
    
    return urls


def extract_filename_from_url(url: Optional[str]) -> str:
    """
    Extract filename from a URL, handling cases with no extension.
    
    Args:
        url: URL string (e.g., "https://s3plus.meituan.net/ipr-public-prod/20240527 161941_rk7l20.png")
        
    Returns:
        Filename string (e.g., "20240527 161941_rk7l20.png" or "20240527 161941_rk7l20")
    """
    if not url:
        return ""
    
    # Remove query parameters if any
    url = url.split('?')[0]
    
    # Extract the last part after the last '/'
    filename = url.split('/')[-1]
    
    return filename


def convert_bson_to_es_type1(bson_doc: Dict[str, Any]) -> Dict[str, Any]:
    """
    Convert a single MongoDB BSON document to Elasticsearch document format.
    This function handles Type 1 problems (Waimai/Takeaway reviews).
    
    Args:
        bson_doc: MongoDB BSON document
        
    Returns:
        Elasticsearch document dictionary
    """
    # Initialize ES document
    es_doc = {}
    
    # Top-level fields
    mongo_id = safe_get(bson_doc, "_id")
    es_doc["mongo_id"] = str(mongo_id) if mongo_id else None
    
    # Stars: detail.taskInfo.voteContent.review.star / 10
    star = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "review", "star", default=0)
    try:
        es_doc["stars"] = int(star) / 10 if star else 0
    except (ValueError, TypeError):
        es_doc["stars"] = 0
    
    # User review
    es_doc["user_review"] = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "review", "reviewContent", default="")
    
    # Review pictures
    review_pics = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "review", "reviewPicList", default=[])
    es_doc["review_pics"] = review_pics if isinstance(review_pics, list) else []
    
    # Timestamp: addTime / 1000 (ms to seconds) - as integer
    add_time = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "review", "addTime")
    try:
        es_doc["timestamp"] = int(int(add_time) / 1000) if add_time else 0
    except (ValueError, TypeError):
        es_doc["timestamp"] = 0
    
    # Others: category list + " in " + city
    poi_categories = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "merchant", "poiCategoryList", default=[])
    city = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "merchant", "city", default="")
    if isinstance(poi_categories, list) and poi_categories:
        categories_str = ",".join(poi_categories)
    else:
        categories_str = ""
    es_doc["others"] = f"{categories_str} in {city}" if city else categories_str
    
    # Problem type: 1 for this function
    es_doc["problem_type"] = 1
    
    # Answer: 1 if newOpposeRatio > newSupportRatio, else 2
    support_ratio = safe_get(bson_doc, "detail", "taskInfo", "newSupportRatio", default="0")
    oppose_ratio = safe_get(bson_doc, "detail", "taskInfo", "newOpposeRatio", default="0")
    try:
        support = int(support_ratio)
        oppose = int(oppose_ratio)
        es_doc["answer"] = 1 if oppose > support else 2
    except (ValueError, TypeError):
        es_doc["answer"] = 2
    
    # Ratios
    try:
        es_doc["ratio_1"] = int(oppose_ratio)
    except (ValueError, TypeError):
        es_doc["ratio_1"] = 0
    
    try:
        es_doc["ratio_2"] = int(support_ratio)
    except (ValueError, TypeError):
        es_doc["ratio_2"] = 0
    
    # Uploader
    es_doc["uploader"] = safe_get(bson_doc, "uploader")
    
    # TaskId and UserId
    es_doc["taskId"] = safe_get(bson_doc, "taskId")
    es_doc["userId"] = safe_get(bson_doc, "userId")
    
    # Created at
    upload_timestamp = safe_get(bson_doc, "upload_timestamp")
    try:
        es_doc["created_at"] = int(upload_timestamp) if upload_timestamp else 0
    except (ValueError, TypeError):
        es_doc["created_at"] = 0
    
    # Replies array
    replies = []
    reply_list = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "replyList", default=[])
    if isinstance(reply_list, list):
        others_counter = 0
        for reply in reply_list:
            if not isinstance(reply, dict):
                continue
            
            reply_type = safe_get(reply, "replyType")
            role = convert_reply_role(reply_type, others_counter)
            if role.startswith("others_"):
                others_counter += 1
            
            reply_time = safe_get(reply, "addTime")
            try:
                timestamp = int(int(reply_time) / 1000) if reply_time else 0
            except (ValueError, TypeError):
                timestamp = 0
            
            replies.append({
                "role": role,
                "timestamp": timestamp,
                "content": safe_get(reply, "replyContent", default="")
            })
    
    # Sort replies by timestamp
    replies.sort(key=lambda x: x["timestamp"])
    
    es_doc["replies"] = replies
    
    # Appeals array
    appeals = []
    evidence_list = safe_get(bson_doc, "detail", "taskInfo", "evidenceList", default=[])
    if isinstance(evidence_list, list):
        for evidence in evidence_list:
            if not isinstance(evidence, dict):
                continue
            
            belong_type = safe_get(evidence, "belongType")
            role = "merchant" if belong_type == "PROSECUTOR_EVIDENCE" else "user"
            
            create_time = safe_get(evidence, "createTime")
            try:
                timestamp = int(int(create_time) / 1000) if create_time else 0
            except (ValueError, TypeError):
                timestamp = 0
            
            # Combine pictures and videos - extract URLs only
            picture_list = safe_get(evidence, "pictureInfoDtoList", default=[])
            video_list = safe_get(evidence, "videoInfoDtoList", default=[])
            pics = extract_pic_urls(picture_list, video_list)
            
            appeals.append({
                "role": role,
                "timestamp": timestamp,
                "content": safe_get(evidence, "txtContent", default=""),
                "pics": pics
            })
    
    es_doc["appeals"] = appeals
    
    # Order info: null for takeaway type
    es_doc["order_info"] = None
    
    # Orders array
    orders = []
    product_list = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "order", "takeawayOrder", "productInfoDtoList", default=[])
    if isinstance(product_list, list):
        for product in product_list:
            if not isinstance(product, dict):
                continue
            
            # Build others field
            spec = safe_get(product, "spec", default="")
            min_order = safe_get(product, "minOrderCount", default="")
            
            others_parts = []
            if spec:
                others_parts.append(spec)
            if min_order:
                others_parts.append(f"Minimum per order: {min_order}")
            others_str = ", ".join(others_parts) if others_parts else ""
            
            # Get selection (attrValueList)
            attr_values = safe_get(product, "attrValueList", default=[])
            selection = attr_values if isinstance(attr_values, list) else []
            
            # Get count
            count = safe_get(product, "count", default=0)
            try:
                count = int(count)
            except (ValueError, TypeError):
                count = 0
            
            orders.append({
                "name": safe_get(product, "name", default=""),
                "count": count,
                "desc": safe_get(product, "description", default=""),
                "selection": selection,
                "pic": safe_get(product, "picUrl", default=""),
                "others": others_str
            })
    
    es_doc["orders"] = orders
    
    # Order detail
    takeaway_order = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "order", "takeawayOrder", default={})
    if isinstance(takeaway_order, dict):
        # Order started - as integer
        order_time = safe_get(takeaway_order, "orderTime")
        try:
            order_started = int(int(order_time) / 1000) if order_time else 0
        except (ValueError, TypeError):
            order_started = 0
        
        # Order finished - as integer
        finish_time = safe_get(takeaway_order, "finishTime")
        try:
            order_finished = int(int(finish_time) / 1000) if finish_time else 0
        except (ValueError, TypeError):
            order_finished = 0
        
        # Deliver time - as integer
        delivery_time_ms = safe_get(takeaway_order, "deliveryTimeTakeMs")
        try:
            deliver_time = int(int(delivery_time_ms) / 1000) if delivery_time_ms else -1
        except (ValueError, TypeError):
            deliver_time = -1
        
        # Total time - as integer
        order_time_ms = safe_get(takeaway_order, "orderTimeTakeMs")
        try:
            total_time = int(int(order_time_ms) / 1000) if order_time_ms else 0
        except (ValueError, TypeError):
            total_time = 0
        
        # Deliver by
        delivery_type_cn = safe_get(takeaway_order, "deliveryTypeCn")
        deliver_by = convert_delivery_type(delivery_type_cn)
        
        # Note
        note = safe_get(takeaway_order, "remarkContent", default="")
        
        # Utensils
        need_tableware = safe_get(takeaway_order, "needTableware", default=False)
        tableware_count = safe_get(takeaway_order, "tablewareCount", default=-1)
        try:
            if need_tableware:
                utensils = int(tableware_count) if tableware_count != -1 else -1
            else:
                utensils = 0
        except (ValueError, TypeError):
            utensils = 0
        
        # Invoice
        invoice = safe_get(takeaway_order, "needInvoice", default=False)
        
        es_doc["order_detail"] = {
            "order_started": order_started,
            "order_finished": order_finished,
            "deliver_time": deliver_time,
            "total_time": total_time,
            "deliver_by": deliver_by,
            "note": note,
            "utensils": utensils,
            "invoice": invoice
        }
    else:
        es_doc["order_detail"] = None
    
    # Comments array
    comments = []
    comment_array = safe_get(bson_doc, "comment", default=[])
    if isinstance(comment_array, list):
        for comment_page in comment_array:
            if not isinstance(comment_page, dict):
                continue
            
            page_content = safe_get(comment_page, "pageContent", default=[])
            if isinstance(page_content, list):
                for comment_item in page_content:
                    if not isinstance(comment_item, dict):
                        continue
                    
                    # Choice: 1 if voteOperate is 'DOWN', else 2
                    vote_operate = safe_get(comment_item, "voteOperate")
                    choice = 1 if vote_operate == "DOWN" else (2 if vote_operate == "UP" else 0)
                    
                    # Timestamp - as integer
                    create_time = safe_get(comment_item, "createTime")
                    try:
                        timestamp = int(int(create_time) / 1000) if create_time else 0
                    except (ValueError, TypeError):
                        timestamp = 0
                    
                    # User ID
                    user_id = safe_get(comment_item, "userId")
                    try:
                        userid = int(user_id) if user_id else 0
                    except (ValueError, TypeError):
                        userid = user_id if user_id else 0
                    
                    # Likes - extract from source if available, default to 0
                    likes = safe_get(comment_item, "approveCount", default=-1)
                    try:
                        likes = int(likes) if likes else 0
                    except (ValueError, TypeError):
                        likes = -1
                    
                    comments.append({
                        "userid": userid,
                        "name": safe_get(comment_item, "userName", default=""),
                        "content": safe_get(comment_item, "content", default=""),
                        "timestamp": timestamp,
                        "choice": choice,
                        "likes": likes
                    })
    
    es_doc["comments"] = comments
    
    return es_doc


def convert_bson_to_es_type2(bson_doc: Dict[str, Any]) -> Dict[str, Any]:
    """
    Convert a single MongoDB BSON document to Elasticsearch document format.
    This function handles Type 2 problems (Dine-in/堂食 reviews).
    
    Args:
        bson_doc: MongoDB BSON document
        
    Returns:
        Elasticsearch document dictionary
    """
    # Initialize ES document
    es_doc = {}
    
    # Top-level fields (same as Type 1)
    mongo_id = safe_get(bson_doc, "_id")
    es_doc["mongo_id"] = str(mongo_id) if mongo_id else None
    
    # Stars: detail.taskInfo.voteContent.review.star / 10
    star = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "review", "star", default=0)
    try:
        es_doc["stars"] = int(star) / 10 if star else 0
    except (ValueError, TypeError):
        es_doc["stars"] = 0
    
    # User review
    es_doc["user_review"] = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "review", "reviewContent", default="")
    
    # Review pictures
    review_pics = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "review", "reviewPicList", default=[])
    es_doc["review_pics"] = review_pics if isinstance(review_pics, list) else []
    
    # Timestamp: addTime / 1000 (ms to seconds) - as integer
    add_time = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "review", "addTime")
    try:
        es_doc["timestamp"] = int(int(add_time) / 1000) if add_time else 0
    except (ValueError, TypeError):
        es_doc["timestamp"] = 0
    
    # Others: category list + " in " + city
    poi_categories = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "merchant", "poiCategoryList", default=[])
    city = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "merchant", "city", default="")
    if isinstance(poi_categories, list) and poi_categories:
        categories_str = ",".join(poi_categories)
    else:
        categories_str = ""
    es_doc["others"] = f"{categories_str} in {city}" if city else categories_str
    
    # Problem type: 2 for this function
    es_doc["problem_type"] = 2
    
    # Answer: 1 if newOpposeRatio > newSupportRatio, else 2
    support_ratio = safe_get(bson_doc, "detail", "taskInfo", "newSupportRatio", default="0")
    oppose_ratio = safe_get(bson_doc, "detail", "taskInfo", "newOpposeRatio", default="0")
    try:
        support = int(support_ratio)
        oppose = int(oppose_ratio)
        es_doc["answer"] = 1 if oppose > support else 2
    except (ValueError, TypeError):
        es_doc["answer"] = 2
    
    # Ratios
    try:
        es_doc["ratio_1"] = int(oppose_ratio)
    except (ValueError, TypeError):
        es_doc["ratio_1"] = 0
    
    try:
        es_doc["ratio_2"] = int(support_ratio)
    except (ValueError, TypeError):
        es_doc["ratio_2"] = 0
    
    # Uploader
    es_doc["uploader"] = safe_get(bson_doc, "uploader")
    
    # TaskId and UserId
    es_doc["taskId"] = safe_get(bson_doc, "taskId")
    es_doc["userId"] = safe_get(bson_doc, "userId")
    
    # Created at
    upload_timestamp = safe_get(bson_doc, "upload_timestamp")
    try:
        es_doc["created_at"] = int(upload_timestamp) if upload_timestamp else 0
    except (ValueError, TypeError):
        es_doc["created_at"] = 0
    
    # Replies array (Type 2 specific: extract number from replyName)
    replies = []
    reply_list = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "replyList", default=[])
    if isinstance(reply_list, list):
        for reply in reply_list:
            if not isinstance(reply, dict):
                continue
            
            reply_type = safe_get(reply, "replyType")
            reply_name = safe_get(reply, "replyName")
            role = convert_reply_role_type2(reply_type, reply_name)
            
            reply_time = safe_get(reply, "addTime")
            try:
                timestamp = int(int(reply_time) / 1000) if reply_time else 0
            except (ValueError, TypeError):
                timestamp = 0
            
            replies.append({
                "role": role,
                "timestamp": timestamp,
                "content": safe_get(reply, "replyContent", default="")
            })
    
    # Sort replies by timestamp
    replies.sort(key=lambda x: x["timestamp"])
    
    es_doc["replies"] = replies
    
    # Appeals array (same as Type 1)
    appeals = []
    evidence_list = safe_get(bson_doc, "detail", "taskInfo", "evidenceList", default=[])
    if isinstance(evidence_list, list):
        for evidence in evidence_list:
            if not isinstance(evidence, dict):
                continue
            
            belong_type = safe_get(evidence, "belongType")
            role = "merchant" if belong_type == "PROSECUTOR_EVIDENCE" else "user"
            
            create_time = safe_get(evidence, "createTime")
            try:
                timestamp = int(int(create_time) / 1000) if create_time else 0
            except (ValueError, TypeError):
                timestamp = 0
            
            # Combine pictures and videos - extract URLs only
            picture_list = safe_get(evidence, "pictureInfoDtoList", default=[])
            video_list = safe_get(evidence, "videoInfoDtoList", default=[])
            pics = extract_pic_urls(picture_list, video_list)
            
            appeals.append({
                "role": role,
                "timestamp": timestamp,
                "content": safe_get(evidence, "txtContent", default=""),
                "pics": pics
            })
    
    es_doc["appeals"] = appeals
    
    # Order info: Type 2 specific structure
    order_raw = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "order")
    poi_name = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "merchant", "poiName")
    poi_avg_price = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "merchant", "poiAvgPrice")
    
    # Convert poiAvgPrice to int if possible
    try:
        avg_price = int(poi_avg_price) if poi_avg_price is not None else 0
    except (ValueError, TypeError):
        avg_price = 0
    
    es_doc["order_info"] = {
        "raw": order_raw,
        "shop_info": {
            "name": poi_name if poi_name else "",
            "avg_price": avg_price
        },
        "processed": None
    }
    
    # Orders array: empty for Type 2
    es_doc["orders"] = []
    
    # Order detail: null for Type 2
    es_doc["order_detail"] = None
    
    # Comments array (same as Type 1)
    comments = []
    comment_array = safe_get(bson_doc, "comment", default=[])
    if isinstance(comment_array, list):
        for comment_page in comment_array:
            if not isinstance(comment_page, dict):
                continue
            
            page_content = safe_get(comment_page, "pageContent", default=[])
            if isinstance(page_content, list):
                for comment_item in page_content:
                    if not isinstance(comment_item, dict):
                        continue
                    
                    # Choice: 1 if voteOperate is 'DOWN', else 2
                    vote_operate = safe_get(comment_item, "voteOperate")
                    choice = 1 if vote_operate == "DOWN" else (2 if vote_operate == "UP" else 0)
                    
                    # Timestamp - as integer
                    create_time = safe_get(comment_item, "createTime")
                    try:
                        timestamp = int(int(create_time) / 1000) if create_time else 0
                    except (ValueError, TypeError):
                        timestamp = 0
                    
                    # User ID
                    user_id = safe_get(comment_item, "userId")
                    try:
                        userid = int(user_id) if user_id else 0
                    except (ValueError, TypeError):
                        userid = user_id if user_id else 0
                    
                    # Likes - extract from source if available, default to 0
                    likes = safe_get(comment_item, "approveCount", default=-1)
                    try:
                        likes = int(likes) if likes else 0
                    except (ValueError, TypeError):
                        likes = -1
                    
                    comments.append({
                        "userid": userid,
                        "name": safe_get(comment_item, "userName", default=""),
                        "content": safe_get(comment_item, "content", default=""),
                        "timestamp": timestamp,
                        "choice": choice,
                        "likes": likes
                    })
    
    es_doc["comments"] = comments
    
    return es_doc


def convert_bson_to_es_type3(bson_doc: Dict[str, Any]) -> Dict[str, Any]:
    """
    Convert a single MongoDB BSON document to Elasticsearch document format.
    This function handles Type 3 problems (Waimai refunds/外卖退款).
    
    Args:
        bson_doc: MongoDB BSON document
        
    Returns:
        Elasticsearch document dictionary
    """
    # Initialize ES document
    es_doc = {}
    
    # Top-level fields
    mongo_id = safe_get(bson_doc, "_id")
    es_doc["mongo_id"] = str(mongo_id) if mongo_id else None
    
    # Stars: For Type 3, no review, so default to 0
    es_doc["stars"] = 0
    
    # User review: Will be set from first appeal after sorting
    # (Temporary placeholder, will be updated after appeals are processed)
    es_doc["user_review"] = ""
    
    # Review pictures: Empty for Type 3
    es_doc["review_pics"] = []
    
    # Others: Show refund price
    refund_sum = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "refundInfo", "takeawayRefundInfoDto", "refundSum", default="")
    
    if refund_sum:
        es_doc["others"] = f"Refund: ¥{refund_sum}"
    else:
        es_doc["others"] = ""
    
    # Problem type: 3 for this function
    es_doc["problem_type"] = 3
    
    # Answer: 1 if newOpposeRatio > newSupportRatio, else 2
    support_ratio = safe_get(bson_doc, "detail", "taskInfo", "newSupportRatio", default="0")
    oppose_ratio = safe_get(bson_doc, "detail", "taskInfo", "newOpposeRatio", default="0")
    try:
        support = int(support_ratio)
        oppose = int(oppose_ratio)
        es_doc["answer"] = 1 if oppose > support else 2
    except (ValueError, TypeError):
        es_doc["answer"] = 2
    
    # Ratios
    try:
        es_doc["ratio_1"] = int(oppose_ratio)
    except (ValueError, TypeError):
        es_doc["ratio_1"] = 0
    
    try:
        es_doc["ratio_2"] = int(support_ratio)
    except (ValueError, TypeError):
        es_doc["ratio_2"] = 0
    
    # Uploader
    es_doc["uploader"] = safe_get(bson_doc, "uploader")
    
    # TaskId and UserId
    es_doc["taskId"] = safe_get(bson_doc, "taskId")
    es_doc["userId"] = safe_get(bson_doc, "userId")
    
    # Created at
    upload_timestamp = safe_get(bson_doc, "upload_timestamp")
    try:
        es_doc["created_at"] = int(upload_timestamp) if upload_timestamp else 0
    except (ValueError, TypeError):
        es_doc["created_at"] = 0
    
    # Replies array: Empty for Type 3
    es_doc["replies"] = []
    
    # Appeals array - sorted by timestamp ASC
    appeals = []
    evidence_list = safe_get(bson_doc, "detail", "taskInfo", "evidenceList", default=[])
    if isinstance(evidence_list, list):
        for evidence in evidence_list:
            if not isinstance(evidence, dict):
                continue
            
            belong_type = safe_get(evidence, "belongType")
            role = "merchant" if belong_type == "PROSECUTOR_EVIDENCE" else "user"
            
            create_time = safe_get(evidence, "createTime")
            try:
                timestamp = int(int(create_time) / 1000) if create_time else 0
            except (ValueError, TypeError):
                timestamp = 0
            
            # Combine pictures and videos - extract URLs only
            picture_list = safe_get(evidence, "pictureInfoDtoList", default=[])
            video_list = safe_get(evidence, "videoInfoDtoList", default=[])
            pics = extract_pic_urls(picture_list, video_list)
            
            appeals.append({
                "role": role,
                "timestamp": timestamp,
                "content": safe_get(evidence, "txtContent", default=""),
                "pics": pics
            })
    
    # Sort appeals by timestamp (ASC)
    appeals.sort(key=lambda x: x["timestamp"])
    
    es_doc["appeals"] = appeals

    # Timestamp: first appeal timestamp after sorting
    es_doc["timestamp"] = appeals[0]["timestamp"] if appeals else 0
    
    # User review: Set from first appeal content (after sorting)
    if appeals and appeals[0].get("content"):
        es_doc["user_review"] = appeals[0]["content"]
    else:
        es_doc["user_review"] = ""
    
    # Order info: null for Type 3
    es_doc["order_info"] = None
    
    # Orders array: From refundProductionList
    orders = []
    refund_product_list = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "refundInfo", "takeawayRefundInfoDto", "refundProductionList", default=[])
    if isinstance(refund_product_list, list):
        for product in refund_product_list:
            if not isinstance(product, dict):
                continue
            
            # Build others field: spec, price, minOrderCount
            spec = safe_get(product, "spec", default="")
            price = safe_get(product, "price", default="")
            min_order = safe_get(product, "minOrderCount", default="")
            
            others_parts = []
            if spec:
                others_parts.append(spec)
            if price:
                others_parts.append(f"Price: {price}")
            if min_order:
                others_parts.append(f"Minimum per order: {min_order}")
            others_str = ", ".join(others_parts) if others_parts else ""
            
            # Get selection (attrValueList)
            attr_values = safe_get(product, "attrValueList", default=[])
            selection = attr_values if isinstance(attr_values, list) else []
            
            # Get count
            count = safe_get(product, "count", default=0)
            try:
                count = int(count)
            except (ValueError, TypeError):
                count = 0
            
            orders.append({
                "name": safe_get(product, "name", default=""),
                "count": count,
                "desc": safe_get(product, "description", default=""),
                "selection": selection,
                "pic": safe_get(product, "picUrl", default=""),
                "others": others_str
            })
    
    es_doc["orders"] = orders
    
    # Order detail: Same as Type 1, from takeawayOrder
    takeaway_order = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "order", "takeawayOrder", default={})
    if isinstance(takeaway_order, dict):
        # Order started - as integer
        order_time = safe_get(takeaway_order, "orderTime")
        try:
            order_started = int(int(order_time) / 1000) if order_time else 0
        except (ValueError, TypeError):
            order_started = 0
        
        # Order finished - as integer
        finish_time = safe_get(takeaway_order, "finishTime")
        try:
            order_finished = int(int(finish_time) / 1000) if finish_time else 0
        except (ValueError, TypeError):
            order_finished = 0
        
        # Deliver time - as integer
        delivery_time_ms = safe_get(takeaway_order, "deliveryTimeTakeMs")
        try:
            deliver_time = int(int(delivery_time_ms) / 1000) if delivery_time_ms else -1
        except (ValueError, TypeError):
            deliver_time = -1
        
        # Total time - as integer
        order_time_ms = safe_get(takeaway_order, "orderTimeTakeMs")
        try:
            total_time = int(int(order_time_ms) / 1000) if order_time_ms else 0
        except (ValueError, TypeError):
            total_time = 0
        
        # Deliver by
        delivery_type_cn = safe_get(takeaway_order, "deliveryTypeCn")
        deliver_by = convert_delivery_type(delivery_type_cn)
        
        # Note
        note = safe_get(takeaway_order, "remarkContent", default="")
        
        # Utensils
        need_tableware = safe_get(takeaway_order, "needTableware", default=False)
        tableware_count = safe_get(takeaway_order, "tablewareCount", default=-1)
        try:
            if need_tableware:
                utensils = int(tableware_count) if tableware_count != -1 else -1
            else:
                utensils = 0
        except (ValueError, TypeError):
            utensils = 0
        
        # Invoice
        invoice = safe_get(takeaway_order, "needInvoice", default=False)
        
        es_doc["order_detail"] = {
            "order_started": order_started,
            "order_finished": order_finished,
            "deliver_time": deliver_time,
            "total_time": total_time,
            "deliver_by": deliver_by,
            "note": note,
            "utensils": utensils,
            "invoice": invoice
        }
    else:
        es_doc["order_detail"] = None
    
    # Comments array (same as Type 1/2)
    comments = []
    comment_array = safe_get(bson_doc, "comment", default=[])
    if isinstance(comment_array, list):
        for comment_page in comment_array:
            if not isinstance(comment_page, dict):
                continue
            
            page_content = safe_get(comment_page, "pageContent", default=[])
            if isinstance(page_content, list):
                for comment_item in page_content:
                    if not isinstance(comment_item, dict):
                        continue
                    
                    # Choice: 1 if voteOperate is 'DOWN', else 2
                    vote_operate = safe_get(comment_item, "voteOperate")
                    choice = 1 if vote_operate == "DOWN" else (2 if vote_operate == "UP" else 0)
                    
                    # Timestamp - as integer
                    create_time = safe_get(comment_item, "createTime")
                    try:
                        timestamp = int(int(create_time) / 1000) if create_time else 0
                    except (ValueError, TypeError):
                        timestamp = 0
                    
                    # User ID
                    user_id = safe_get(comment_item, "userId")
                    try:
                        userid = int(user_id) if user_id else 0
                    except (ValueError, TypeError):
                        userid = user_id if user_id else 0
                    
                    # Likes - extract from source if available, default to 0
                    likes = safe_get(comment_item, "approveCount", default=-1)
                    try:
                        likes = int(likes) if likes else 0
                    except (ValueError, TypeError):
                        likes = -1
                    
                    comments.append({
                        "userid": userid,
                        "name": safe_get(comment_item, "userName", default=""),
                        "content": safe_get(comment_item, "content", default=""),
                        "timestamp": timestamp,
                        "choice": choice,
                        "likes": likes
                    })
    
    es_doc["comments"] = comments
    
    return es_doc


def convert_bson_to_es_type4(bson_doc: Dict[str, Any]) -> Dict[str, Any]:
    """
    Convert a single MongoDB BSON document to Elasticsearch document format.
    This function handles Type 4 problems (Dine-in refunds/堂食退款).
    
    Args:
        bson_doc: MongoDB BSON document
        
    Returns:
        Elasticsearch document dictionary
    """
    # Initialize ES document
    es_doc = {}
    
    # Top-level fields
    mongo_id = safe_get(bson_doc, "_id")
    es_doc["mongo_id"] = str(mongo_id) if mongo_id else None
    
    # Stars: No review for refunds, default to 0
    es_doc["stars"] = 0
    
    # User review: Will be set from first appeal after sorting
    # (Temporary placeholder, will be updated after appeals are processed)
    es_doc["user_review"] = ""
    
    # Review pictures: Empty for Type 4
    es_doc["review_pics"] = []
    
    # Timestamp will be set after processing appeals (first appeal timestamp)
    
    # Others: Show refund price
    order_price = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "order", "snapshotOrder", "orderPrice")
    if order_price:
        es_doc["others"] = f"Refund: ¥{order_price}"
    else:
        es_doc["others"] = ""
    
    # Problem type: 4 for this function
    es_doc["problem_type"] = 4
    
    # Answer: 1 if newSupportRatio > newOpposeRatio, else 2 (reversed for Type 4)
    support_ratio = safe_get(bson_doc, "detail", "taskInfo", "newSupportRatio", default="0")
    oppose_ratio = safe_get(bson_doc, "detail", "taskInfo", "newOpposeRatio", default="0")
    try:
        support = int(support_ratio)
        oppose = int(oppose_ratio)
        es_doc["answer"] = 1 if support > oppose else 2
    except (ValueError, TypeError):
        es_doc["answer"] = 2
    
    # Ratios (reversed for Type 4)
    try:
        es_doc["ratio_1"] = int(support_ratio)
    except (ValueError, TypeError):
        es_doc["ratio_1"] = 0
    
    try:
        es_doc["ratio_2"] = int(oppose_ratio)
    except (ValueError, TypeError):
        es_doc["ratio_2"] = 0
    
    # Uploader
    es_doc["uploader"] = safe_get(bson_doc, "uploader")
    
    # TaskId and UserId
    es_doc["taskId"] = safe_get(bson_doc, "taskId")
    es_doc["userId"] = safe_get(bson_doc, "userId")
    
    # Created at
    upload_timestamp = safe_get(bson_doc, "upload_timestamp")
    try:
        es_doc["created_at"] = int(upload_timestamp) if upload_timestamp else 0
    except (ValueError, TypeError):
        es_doc["created_at"] = 0
    
    # Replies array: Empty for Type 4
    es_doc["replies"] = []
    
    # Appeals array - sorted by timestamp ASC
    appeals = []
    evidence_list = safe_get(bson_doc, "detail", "taskInfo", "evidenceList", default=[])
    if isinstance(evidence_list, list):
        for evidence in evidence_list:
            if not isinstance(evidence, dict):
                continue
            
            belong_type = safe_get(evidence, "belongType")
            role = "user" if belong_type == "PROSECUTOR_EVIDENCE" else "merchant"
            
            create_time = safe_get(evidence, "createTime")
            try:
                timestamp = int(int(create_time) / 1000) if create_time else 0
            except (ValueError, TypeError):
                timestamp = 0
            
            # Combine pictures and videos - extract URLs only
            picture_list = safe_get(evidence, "pictureInfoDtoList", default=[])
            video_list = safe_get(evidence, "videoInfoDtoList", default=[])
            pics = extract_pic_urls(picture_list, video_list)
            
            appeals.append({
                "role": role,
                "timestamp": timestamp,
                "content": safe_get(evidence, "txtContent", default=""),
                "pics": pics
            })
    
    # Sort appeals by timestamp (ASC)
    appeals.sort(key=lambda x: x["timestamp"])
    
    es_doc["appeals"] = appeals
    
    # Timestamp: first appeal timestamp after sorting
    es_doc["timestamp"] = appeals[0]["timestamp"] if appeals else 0
    
    # User review: Set from first appeal content (after sorting)
    if appeals and appeals[0].get("content"):
        es_doc["user_review"] = appeals[0]["content"]
    else:
        es_doc["user_review"] = ""
    
    # Order info: Type 4 specific structure
    snapshot_order = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "order", "snapshotOrder")
    
    es_doc["order_info"] = {
        "raw": snapshot_order,
        "processed": None
    }
    
    # Orders array: empty for Type 4
    es_doc["orders"] = []
    
    # Order detail: null for Type 4
    es_doc["order_detail"] = None
    
    # Comments array (reversed choice for Type 4)
    comments = []
    comment_array = safe_get(bson_doc, "comment", default=[])
    if isinstance(comment_array, list):
        for comment_page in comment_array:
            if not isinstance(comment_page, dict):
                continue
            
            page_content = safe_get(comment_page, "pageContent", default=[])
            if isinstance(page_content, list):
                for comment_item in page_content:
                    if not isinstance(comment_item, dict):
                        continue
                    
                    # Choice: 2 if voteOperate is 'DOWN', else 1 (reversed for Type 4)
                    vote_operate = safe_get(comment_item, "voteOperate")
                    choice = 2 if vote_operate == "DOWN" else (1 if vote_operate == "UP" else 0)
                    
                    # Timestamp - as integer
                    create_time = safe_get(comment_item, "createTime")
                    try:
                        timestamp = int(int(create_time) / 1000) if create_time else 0
                    except (ValueError, TypeError):
                        timestamp = 0
                    
                    # User ID
                    user_id = safe_get(comment_item, "userId")
                    try:
                        userid = int(user_id) if user_id else 0
                    except (ValueError, TypeError):
                        userid = user_id if user_id else 0
                    
                    # Likes - extract from source if available, default to 0
                    likes = safe_get(comment_item, "approveCount", default=-1)
                    try:
                        likes = int(likes) if likes else 0
                    except (ValueError, TypeError):
                        likes = -1
                    
                    comments.append({
                        "userid": userid,
                        "name": safe_get(comment_item, "userName", default=""),
                        "content": safe_get(comment_item, "content", default=""),
                        "timestamp": timestamp,
                        "choice": choice,
                        "likes": likes
                    })
    
    es_doc["comments"] = comments
    
    return es_doc


def convert_bson_to_es_type5(bson_doc: Dict[str, Any]) -> Dict[str, Any]:
    """
    Convert a single MongoDB BSON document to Elasticsearch document format.
    This function handles Type 5 problems (IPR).
    
    Args:
        bson_doc: MongoDB BSON document
        
    Returns:
        Elasticsearch document dictionary
    """
    # Initialize ES document
    es_doc = {}
    
    # Top-level fields (same as Type 4)
    mongo_id = safe_get(bson_doc, "_id")
    es_doc["mongo_id"] = str(mongo_id) if mongo_id else None
    
    # Stars: No review for IPR, default to 0
    es_doc["stars"] = 0
    
    # Get logo URLs
    prosecutor_logo_url = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "logoInfo", "prosecutorLogoUrl", default="")
    defendant_logo_url = safe_get(bson_doc, "detail", "taskInfo", "voteContent", "logoInfo", "defendantLogoUrl", default="")
    
    # Extract filenames from URLs
    filename1 = extract_filename_from_url(prosecutor_logo_url)
    filename2 = extract_filename_from_url(defendant_logo_url)
    
    # User review: "{filename1} vs {filename2}"
    es_doc["user_review"] = f"{filename1} vs {filename2}"
    
    # Review pictures: [prosecutorLogoUrl, defendantLogoUrl]
    review_pics = []
    if prosecutor_logo_url:
        review_pics.append(prosecutor_logo_url)
    if defendant_logo_url:
        review_pics.append(defendant_logo_url)
    es_doc["review_pics"] = review_pics
    
    # Timestamp: int(detail.taskInfo.voteEndTime / 1000)
    vote_end_time = safe_get(bson_doc, "detail", "taskInfo", "voteEndTime")
    try:
        es_doc["timestamp"] = int(int(vote_end_time) / 1000) if vote_end_time else 0
    except (ValueError, TypeError):
        es_doc["timestamp"] = 0
    
    # Others: empty string
    es_doc["others"] = ""
    
    # Problem type: 5 for this function
    es_doc["problem_type"] = 5
    
    # Answer: 1 if newSupportRatio > newOpposeRatio, else 2 (same as Type 4)
    support_ratio = safe_get(bson_doc, "detail", "taskInfo", "newSupportRatio", default="0")
    oppose_ratio = safe_get(bson_doc, "detail", "taskInfo", "newOpposeRatio", default="0")
    try:
        support = int(support_ratio)
        oppose = int(oppose_ratio)
        es_doc["answer"] = 1 if support > oppose else 2
    except (ValueError, TypeError):
        es_doc["answer"] = 2
    
    # Ratios (same as Type 4: ratio_1 is support)
    try:
        es_doc["ratio_1"] = int(support_ratio)
    except (ValueError, TypeError):
        es_doc["ratio_1"] = 0
    
    try:
        es_doc["ratio_2"] = int(oppose_ratio)
    except (ValueError, TypeError):
        es_doc["ratio_2"] = 0
    
    # Uploader
    es_doc["uploader"] = safe_get(bson_doc, "uploader")
    
    # TaskId and UserId
    es_doc["taskId"] = safe_get(bson_doc, "taskId")
    es_doc["userId"] = safe_get(bson_doc, "userId")
    
    # Created at
    upload_timestamp = safe_get(bson_doc, "upload_timestamp")
    try:
        es_doc["created_at"] = int(upload_timestamp) if upload_timestamp else 0
    except (ValueError, TypeError):
        es_doc["created_at"] = 0
    
    # Replies array: Empty for Type 5
    es_doc["replies"] = []
    
    # Appeals array: Empty for Type 5
    es_doc["appeals"] = []
    
    # Order info: null for Type 5
    es_doc["order_info"] = None
    
    # Orders array: Empty for Type 5
    es_doc["orders"] = []
    
    # Order detail: null for Type 5
    es_doc["order_detail"] = None
    
    # Comments array (same as Type 4: reversed choice)
    comments = []
    comment_array = safe_get(bson_doc, "comment", default=[])
    if isinstance(comment_array, list):
        for comment_page in comment_array:
            if not isinstance(comment_page, dict):
                continue
            
            page_content = safe_get(comment_page, "pageContent", default=[])
            if isinstance(page_content, list):
                for comment_item in page_content:
                    if not isinstance(comment_item, dict):
                        continue
                    
                    # Choice: 2 if voteOperate is 'DOWN', else 1 (reversed for Type 5, same as Type 4)
                    vote_operate = safe_get(comment_item, "voteOperate")
                    choice = 2 if vote_operate == "DOWN" else (1 if vote_operate == "UP" else 0)
                    
                    # Timestamp - as integer
                    create_time = safe_get(comment_item, "createTime")
                    try:
                        timestamp = int(int(create_time) / 1000) if create_time else 0
                    except (ValueError, TypeError):
                        timestamp = 0
                    
                    # User ID
                    user_id = safe_get(comment_item, "userId")
                    try:
                        userid = int(user_id) if user_id else 0
                    except (ValueError, TypeError):
                        userid = user_id if user_id else 0
                    
                    # Likes - extract from source if available, default to 0
                    likes = safe_get(comment_item, "approveCount", default=-1)
                    try:
                        likes = int(likes) if likes else 0
                    except (ValueError, TypeError):
                        likes = -1
                    
                    comments.append({
                        "userid": userid,
                        "name": safe_get(comment_item, "userName", default=""),
                        "content": safe_get(comment_item, "content", default=""),
                        "timestamp": timestamp,
                        "choice": choice,
                        "likes": likes
                    })
    
    es_doc["comments"] = comments
    
    return es_doc


def convert_bson_to_es(bson_doc: Dict[str, Any]) -> Dict[str, Any]:
    """
    Auto-detect problem type and convert MongoDB BSON document to Elasticsearch format.
    
    This is the main entry point that automatically routes to the appropriate
    type-specific converter based on the processType field.
    
    Type mapping:
    - Type 1: WAIMAI_COMMENT (Takeaway reviews)
    - Type 2: DAODIAN_MEAL, STAR_COMMENT, MINSU_COMMENT, DAODIAN_HOTEL, 
              DAODIAN_TICKET, DAODIAN_COMPLEX (Dine-in and service reviews)
    - Type 3: WAIMAI_QUIK_REFUND (Takeaway refunds)
    - Type 4: DAOZONG_JIAOYI (Dine-in refunds)
    - Type 5: IPR (IPR problems)
    
    Args:
        bson_doc: MongoDB BSON document
        
    Returns:
        Elasticsearch document dictionary
    """
    # Get processType from the document
    process_type = safe_get(bson_doc, "detail", "taskInfo", "processType", default="")
    
    # Type 1: Takeaway reviews
    if process_type == "WAIMAI_COMMENT":
        return convert_bson_to_es_type1(bson_doc)
    
    # Type 2: Dine-in and service reviews (multiple categories)
    elif process_type in [
        "DAODIAN_MEAL",
        "STAR_COMMENT",
        "MINSU_COMMENT",
        "DAODIAN_HOTEL",
        "DAODIAN_TICKET",
        "DAODIAN_COMPLEX"
    ]:
        return convert_bson_to_es_type2(bson_doc)
    
    # Type 3: Takeaway refunds
    elif process_type == "WAIMAI_QUIK_REFUND":
        return convert_bson_to_es_type3(bson_doc)
    
    # Type 4: Dine-in refunds
    elif process_type == "DAOZONG_JIAOYI":
        return convert_bson_to_es_type4(bson_doc)
    
    # Type 5: IPR
    elif process_type == "IPR":
        return convert_bson_to_es_type5(bson_doc)
    
    # Unknown processType - default to Type 2 (most flexible)
    else:
        # Log warning but continue with Type 2 as fallback
        import warnings
        warnings.warn(
            f"Unknown processType '{process_type}'. Defaulting to Type 2 converter. "
            f"Known types: WAIMAI_COMMENT, DAODIAN_MEAL, STAR_COMMENT, MINSU_COMMENT, "
            f"DAODIAN_HOTEL, DAODIAN_TICKET, DAODIAN_COMPLEX, WAIMAI_QUIK_REFUND, "
            f"DAOZONG_JIAOYI, IPR",
            UserWarning
        )
        return convert_bson_to_es_type2(bson_doc)


if __name__ == "__main__":
    # Test function will be in a separate file
    pass
