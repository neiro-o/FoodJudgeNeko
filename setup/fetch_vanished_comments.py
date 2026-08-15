#!/usr/bin/env python3
"""Export a user's high-liked comments that no longer exist in problems."""

import argparse
import csv
import sys
from collections import defaultdict
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any, Dict, Iterable, List, Mapping, Optional, Sequence, Set, Tuple
from urllib.parse import urlencode

import yaml
from bson import ObjectId
from elasticsearch import Elasticsearch
from pymongo import MongoClient


SGT = timezone(timedelta(hours=8), name="SGT")
CSV_FIELDS = [
    "problemId",
    "user_review",
    "timestamp",
    "commentId",
    "content",
    "likes",
    "createTime",
    "url",
]
PROBLEM_URL = "https://zqt.meituan.com/xiaomei/vote/jury/api/r/rediectByScene"
MIN_APPROVE_COUNT = 35


def load_config() -> Dict[str, Any]:
    """Load the repository's config.yml."""
    config_path = Path(__file__).resolve().parent.parent / "config.yml"
    if not config_path.exists():
        raise FileNotFoundError(f"config.yml not found: {config_path}")

    with config_path.open("r", encoding="utf-8") as config_file:
        config = yaml.safe_load(config_file) or {}

    mongo_config = config.get("mongodb") or {}
    required_keys = ("connection_string", "database_name", "collections")
    missing = [key for key in required_keys if key not in mongo_config]
    if missing:
        raise KeyError(f"mongodb config missing: {', '.join(missing)}")
    return config


def parse_user_id(value: str) -> str:
    """Validate a numeric user ID and normalize it to the stored string form."""
    try:
        return str(int(value))
    except (TypeError, ValueError) as exc:
        raise argparse.ArgumentTypeError("user_id must be an integer") from exc


def format_create_time_sgt(value: Any) -> str:
    """Convert seconds, milliseconds, or a BSON datetime to an SGT ISO string."""
    if value is None or isinstance(value, bool):
        return ""

    parsed: Optional[datetime] = None
    if isinstance(value, datetime):
        parsed = value
        if parsed.tzinfo is None:
            # PyMongo returns BSON dates as naive UTC datetimes by default.
            parsed = parsed.replace(tzinfo=timezone.utc)
    else:
        try:
            timestamp = float(value)
            if abs(timestamp) >= 100_000_000_000:
                timestamp /= 1000
            parsed = datetime.fromtimestamp(timestamp, tz=timezone.utc)
        except (OSError, OverflowError, TypeError, ValueError):
            return ""

    return parsed.astimezone(SGT).isoformat(sep=" ", timespec="seconds")


def nested_comment_ids(problem: Mapping[str, Any]) -> Set[str]:
    """Collect comment[].pageContent[].commentId values from one problem."""
    result: Set[str] = set()
    comment_pages = problem.get("comment")
    if not isinstance(comment_pages, list):
        return result

    for page in comment_pages:
        if not isinstance(page, Mapping):
            continue
        page_content = page.get("pageContent")
        if not isinstance(page_content, list):
            continue
        for item in page_content:
            if not isinstance(item, Mapping):
                continue
            comment_id = item.get("commentId")
            if comment_id is not None:
                result.add(str(comment_id))
    return result


def batched(values: Sequence[Any], batch_size: int) -> Iterable[Sequence[Any]]:
    for start in range(0, len(values), batch_size):
        yield values[start:start + batch_size]


def build_problem_url(user_id: Any, task_id: Any) -> str:
    """Build the original Xiaomei shared-task URL from a problem document."""
    query = urlencode([
        ("jumpScene", "mockTaskShare"),
        ("userId", str(user_id or "")),
        ("channel", "mockTaskShare"),
        ("encryptMockTaskNo", str(task_id or "")),
    ])
    return f"{PROBLEM_URL}?{query}"


def connect_elasticsearch(config: Mapping[str, Any]) -> Elasticsearch:
    """Create and verify an Elasticsearch client from config.yml."""
    es_config = config.get("elasticsearch") or {}
    hosts = es_config.get("hosts", ["http://localhost:9200"])
    client_config: Dict[str, Any] = {
        "hosts": hosts,
        "request_timeout": 30,
    }
    username = es_config.get("username", "")
    password = es_config.get("password", "")
    if username and password:
        client_config["basic_auth"] = (username, password)
    if any(str(host).lower().startswith("https://") for host in hosts):
        client_config["verify_certs"] = False
        client_config["ssl_show_warn"] = False

    client = Elasticsearch(**client_config)
    if not client.ping():
        client.close()
        raise ConnectionError("Elasticsearch ping returned False")
    return client


def fetch_es_documents(
    es_client: Any,
    index_name: str,
    problem_ids: Sequence[str],
    batch_size: int = 500,
) -> Dict[str, Mapping[str, Any]]:
    """Fetch the ES fields needed by the CSV, keyed by MongoDB problem ID."""
    result: Dict[str, Mapping[str, Any]] = {}
    unique_problem_ids = list(dict.fromkeys(problem_ids))
    for problem_id_batch in batched(unique_problem_ids, batch_size):
        response = es_client.search(
            index=index_name,
            body={
                "query": {"terms": {"mongo_id": list(problem_id_batch)}},
                "_source": ["mongo_id", "user_review", "timestamp", "appeals.content"],
                "size": len(problem_id_batch),
            },
        )
        response_body = getattr(response, "body", response)
        for hit in response_body.get("hits", {}).get("hits", []):
            source = hit.get("_source") or {}
            mongo_id = source.get("mongo_id")
            if mongo_id is not None:
                result[str(mongo_id)] = source
    return result


def es_user_review(source: Mapping[str, Any]) -> str:
    """Use user_review, falling back only to appeals[0].content when empty."""
    user_review = source.get("user_review")
    if isinstance(user_review, str) and user_review.strip():
        return user_review

    appeals = source.get("appeals")
    if isinstance(appeals, list) and appeals and isinstance(appeals[0], Mapping):
        content = appeals[0].get("content")
        if content is not None:
            return str(content)
    return ""


def enrich_rows_with_es(
    rows: Iterable[Mapping[str, Any]],
    es_documents: Mapping[str, Mapping[str, Any]],
) -> List[Dict[str, Any]]:
    """Add ES review/timestamp fields and return final CSV-shaped rows."""
    enriched: List[Dict[str, Any]] = []
    for row in rows:
        problem_id = str(row["problemId"])
        source = es_documents.get(problem_id, {})
        enriched.append({
            "problemId": problem_id,
            "user_review": es_user_review(source),
            "timestamp": format_create_time_sgt(source.get("timestamp")),
            "commentId": row.get("commentId", ""),
            "content": row.get("content", ""),
            "likes": row.get("likes", ""),
            "createTime": row.get("createTime", ""),
            "url": row.get("url", ""),
        })
    return enriched


def find_vanished_comments(
    comments_collection: Any,
    problems_collection: Any,
    user_id: str,
    batch_size: int = 500,
) -> Tuple[List[Dict[str, Any]], int]:
    """Return vanished-comment CSV rows and the number of candidates checked."""
    comment_filter = {"userId": user_id, "approveCount": {"$gt": MIN_APPROVE_COUNT}}
    comment_projection = {
        "_id": 0,
        "problemId": 1,
        "commentId": 1,
        "content": 1,
        "approveCount": 1,
        "createTime": 1,
    }

    candidates_by_problem: Dict[str, List[Dict[str, Any]]] = defaultdict(list)
    candidate_count = 0
    for comment in comments_collection.find(comment_filter, comment_projection):
        candidate_count += 1
        candidates_by_problem[str(comment.get("problemId") or "")].append(comment)

    valid_problem_ids: Dict[str, ObjectId] = {}
    for problem_id in candidates_by_problem:
        if ObjectId.is_valid(problem_id):
            valid_problem_ids[problem_id] = ObjectId(problem_id)

    problems_by_id: Dict[str, Mapping[str, Any]] = {}
    object_ids = list(valid_problem_ids.values())
    problem_projection = {"userId": 1, "taskId": 1, "comment.pageContent.commentId": 1}
    for object_id_batch in batched(object_ids, batch_size):
        cursor = problems_collection.find(
            {"_id": {"$in": list(object_id_batch)}},
            problem_projection,
        )
        for problem in cursor:
            problems_by_id[str(problem["_id"])] = problem

    vanished: List[Dict[str, Any]] = []
    for problem_id, comments in candidates_by_problem.items():
        problem = problems_by_id.get(problem_id)
        # A missing/invalid problemId cannot prove that only the comment
        # vanished, so it is intentionally excluded from this report.
        if problem is None:
            continue
        existing_comment_ids = nested_comment_ids(problem)

        for comment in comments:
            comment_id = str(comment.get("commentId") or "")
            if comment_id in existing_comment_ids:
                continue
            vanished.append({
                "problemId": problem_id,
                "commentId": comment_id,
                "content": comment.get("content", ""),
                "likes": comment.get("approveCount", ""),
                "createTime": format_create_time_sgt(comment.get("createTime")),
                "url": build_problem_url(problem.get("userId"), problem.get("taskId")),
            })

    vanished.sort(
        key=lambda row: (
            -(float(row["likes"]) if isinstance(row["likes"], (int, float)) else 0),
            row["problemId"],
            row["commentId"],
        )
    )
    return vanished, candidate_count


def write_csv(rows: Iterable[Mapping[str, Any]], output_path: Path) -> None:
    output_path.parent.mkdir(parents=True, exist_ok=True)
    with output_path.open("w", encoding="utf-8-sig", newline="") as csv_file:
        writer = csv.DictWriter(csv_file, fieldnames=CSV_FIELDS)
        writer.writeheader()
        writer.writerows(rows)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description=f"Export approveCount > {MIN_APPROVE_COUNT} comments missing from problems.",
    )
    parser.add_argument("user_id", type=parse_user_id, help="numeric comments.userId")
    parser.add_argument(
        "-o",
        "--output",
        type=Path,
        help="CSV output path (default: <user_id>.csv)",
    )
    parser.add_argument(
        "--batch-size",
        type=int,
        default=500,
        help="number of problem ObjectIds queried per batch (default: 500)",
    )
    return parser


def main() -> int:
    args = build_parser().parse_args()
    if args.batch_size < 1:
        print("error: --batch-size must be at least 1", file=sys.stderr)
        return 2

    output_path = args.output or Path(f"{args.user_id}.csv")
    client: Optional[MongoClient] = None
    es_client: Optional[Elasticsearch] = None
    try:
        config = load_config()
        mongo_config = config["mongodb"]
        collections = mongo_config["collections"]
        client = MongoClient(
            mongo_config["connection_string"],
            serverSelectionTimeoutMS=5000,
        )
        client.admin.command("ping")
        database = client[mongo_config["database_name"]]
        rows, candidate_count = find_vanished_comments(
            database[collections.get("comments", "comments")],
            database[collections.get("problems", "problems")],
            args.user_id,
            args.batch_size,
        )
        if rows:
            es_client = connect_elasticsearch(config)
            es_config = config.get("elasticsearch") or {}
            es_documents = fetch_es_documents(
                es_client,
                es_config.get("index_name", "problems"),
                [row["problemId"] for row in rows],
                args.batch_size,
            )
            rows = enrich_rows_with_es(rows, es_documents)
        write_csv(rows, output_path)
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1
    finally:
        if client is not None:
            client.close()
        if es_client is not None:
            es_client.close()

    print(f"Checked {candidate_count} comments with approveCount > {MIN_APPROVE_COUNT}.")
    print(f"Found {len(rows)} vanished comments.")
    print(f"CSV written to: {output_path.resolve()}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
