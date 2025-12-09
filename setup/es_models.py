#!/usr/bin/env python3
"""
Elasticsearch data models for mtv2 backend.
Based on ES_Params.md specification.
"""

from dataclasses import dataclass, field
from typing import List, Optional, Union


@dataclass
class Reply:
    """Reply in the replies array"""
    role: str  # merchant, user, or others_X
    timestamp: int  # timestamp in seconds
    content: str


@dataclass
class Appeal:
    """Appeal in the appeals array"""
    role: str  # merchant, user, or others_X
    timestamp: int  # timestamp in seconds
    content: str
    pics: List[str] = field(default_factory=list)  # array of image URLs


@dataclass
class Order:
    """Order item in the orders array"""
    name: str
    count: int
    desc: str
    selection: List[str] = field(default_factory=list)
    pic: str = ""
    others: str = ""


@dataclass
class OrderDetail:
    """Order detail for delivery scenarios"""
    order_started: int  # timestamp in seconds
    order_finished: int  # timestamp in seconds, 0 for pickup
    deliver_time: int  # delivery time in seconds, -1 for unavailable
    total_time: int  # total time from order to completion in seconds
    deliver_by: str  # merchant, meituan, or user
    note: str = ""
    utensils: int = 0  # 0 for eco-friendly, -1 for on-demand, other for specified
    invoice: bool = False


@dataclass
class Comment:
    """Comment in the comments array"""
    userid: int
    name: str
    content: str
    timestamp: int  # timestamp in seconds
    choice: int  # 1 for support user, 2 for support merchant
    likes: int = 0  # number of likes


@dataclass
class ProblemDocument:
    """
    Main Elasticsearch document model for problems.
    Represents a review evaluation record.
    """
    id: Union[str, int]  # unique identifier for the review record
    mongo_id: str  # MongoDB unique identifier
    stars: int = 0  # user rating, 1-5, 1 is worst, 5 is best
    user_review: str = ""  # user's text review, may be empty for refund problems
    review_pics: List[str] = field(default_factory=list)  # array of review image URLs
    timestamp: int = 0  # user review timestamp in seconds
    others: str = ""  # additional notes, e.g., "消费后评价"
    problem_type: int = 0  # 1-delivery, 2-dine-in, 3-delivery refund, 4-dine-in refund, 5-other
    answer: int = 0  # review result: 1-support user, 2-support merchant
    ratio_1: float = 0.0  # ratio of choice 1 (0~100)
    ratio_2: float = 0.0  # ratio of choice 2 (0~100)
    uploader: str = ""  # ID of user who uploaded this problem
    taskId: str = ""  # associated task ID
    userId: str = ""  # original link's userID parameter
    created_at: int = 0  # creation timestamp in seconds
    
    # Nested structures
    replies: List[Reply] = field(default_factory=list)  # timeline of merchant/user replies
    appeals: List[Appeal] = field(default_factory=list)  # appeals from user/merchant
    order_info: Optional[dict] = None  # order info for non-delivery problems (e.g., group purchase info), variable dict
    orders: List[Order] = field(default_factory=list)  # order items
    order_detail: Optional[OrderDetail] = None  # order detail for delivery scenarios
    comments: List[Comment] = field(default_factory=list)  # evaluation comments
    
    def to_dict(self) -> dict:
        """Convert the document to a dictionary for Elasticsearch indexing"""
        result = {
            "id": self.id,
            "mongo_id": self.mongo_id,
            "stars": self.stars,
            "user_review": self.user_review,
            "review_pics": self.review_pics,
            "timestamp": self.timestamp,
            "others": self.others,
            "problem_type": self.problem_type,
            "answer": self.answer,
            "ratio_1": self.ratio_1,
            "ratio_2": self.ratio_2,
            "uploader": self.uploader,
            "taskId": self.taskId,
            "userId": self.userId,
            "created_at": self.created_at,
        }
        
        # Add nested structures if they exist
        if self.replies:
            result["replies"] = [
                {
                    "role": r.role,
                    "timestamp": r.timestamp,
                    "content": r.content
                }
                for r in self.replies
            ]
        
        if self.appeals:
            result["appeals"] = [
                {
                    "role": a.role,
                    "timestamp": a.timestamp,
                    "content": a.content,
                    "pics": a.pics
                }
                for a in self.appeals
            ]
        
        if self.order_info is not None:
            result["order_info"] = self.order_info
        
        if self.orders:
            result["orders"] = [
                {
                    "name": o.name,
                    "count": o.count,
                    "desc": o.desc,
                    "selection": o.selection,
                    "pic": o.pic,
                    "others": o.others
                }
                for o in self.orders
            ]
        
        if self.order_detail is not None:
            result["order_detail"] = {
                "order_started": self.order_detail.order_started,
                "order_finished": self.order_detail.order_finished,
                "deliver_time": self.order_detail.deliver_time,
                "total_time": self.order_detail.total_time,
                "deliver_by": self.order_detail.deliver_by,
                "note": self.order_detail.note,
                "utensils": self.order_detail.utensils,
                "invoice": self.order_detail.invoice
            }
        
        if self.comments:
            result["comments"] = [
                {
                    "userid": c.userid,
                    "name": c.name,
                    "content": c.content,
                    "timestamp": c.timestamp,
                    "choice": c.choice,
                    "likes": c.likes
                }
                for c in self.comments
            ]
        
        # Aggregate searchable content from user_review, replies.content, and appeals.content
        # This allows simple keyword search across all these fields
        searchable_parts = []
        if self.user_review:
            searchable_parts.append(self.user_review)
        if self.replies:
            searchable_parts.extend([r.content for r in self.replies if r.content])
        if self.appeals:
            searchable_parts.extend([a.content for a in self.appeals if a.content])
        
        result["searchable_content"] = " ".join(searchable_parts)
        
        return result

