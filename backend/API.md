# API Documentation

## Health Check

**GET** `/health`

Check if server is running.

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {"status": "ok"}
}
```

---

## Authentication (Public)

### Login

**POST** `/api/login`

Login with username and password.

**Request:**
```json
{
  "username": "user123",
  "password": "password123"
}
```

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "token": "jwt_token_here",
    "user": {
      "id": "user_id",
      "username": "user123",
      "email": "user@example.com",
      "points": 1000,
      "is_admin": false
    }
  }
}
```

### Register

**POST** `/api/register`

Create new account with invitation code.

**Request:**
```json
{
  "username": "newuser",
  "password": "password123",
  "email": "user@example.com",
  "invite_code": "invitation_code_here"
}
```

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "message": "Account created successfully",
    "user": {
      "id": "user_id",
      "username": "newuser",
      "email": "user@example.com",
      "points": 0,
      "is_admin": false
    }
  }
}
```

---

## Authentication (Protected)

All endpoints below require `Authorization: Bearer <token>` header.

### Logout

**POST** `/api/logout`

Logout (client should discard token).

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {"message": "Logged out successfully"}
}
```

### Change Password

**POST** `/api/change-password`

Change user password. Invalidates all existing tokens.

**Request:**
```json
{
  "old_password": "oldpass123",
  "new_password": "newpass123"
}
```

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {"message": "Password changed successfully. Please login again."}
}
```

---

## Invitations (Protected)

### Generate Invitation Codes

**POST** `/api/invitations/generate`

Generate one or more invitation codes. Costs 100 points per code (free for admins).

**Request:**
```json
{
  "count": 1
}
```

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "message": "Invitation codes generated successfully",
    "codes": ["code1", "code2"],
    "count": 2
  }
}
```

### List Invitations

**GET** `/api/invitations`

Get all invitation codes created by current user.

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "points": 1000,
    "invitations": [
      {
        "id": "invitation_id",
        "invite_code": "code_here",
        "created_by": "user_id",
        "created_at": "2024-01-01T00:00:00Z",
        "used": false,
        "used_by": null
      }
    ]
  }
}
```

---

## Problems (Protected)

### Upload Single Problem

**POST** `/api/problem/upload`

Upload a single problem to Redis queue.

**Request:**
```json
{
  "userId": "user123",
  "taskId": "task456"
}
```

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "message": "Problem uploaded successfully",
    "data": {
      "userId": "user123",
      "taskId": "task456",
      "uploadIP": "127.0.0.1"
    }
  }
}
```

### Upload Multiple Problems

**POST** `/api/problem/upload-multiple`

Upload multiple problems to Redis queue in one request.

**Request:**
```json
{
  "problems": [
    {"userId": "user1", "taskId": "task1"},
    {"userId": "user2", "taskId": "task2"},
    {"userId": "user3", "taskId": "task3"}
  ]
}
```

**Response:**
```json
{
  "code": 0,
  "message": "Bulk upload completed: 2 successful, 1 failed",
  "data": {
    "total": 3,
    "success": 2,
    "failed": 1,
    "results": [
      {
        "userId": "user1",
        "taskId": "task1",
        "success": true,
        "message": "Problem uploaded successfully"
      },
      {
        "userId": "user2",
        "taskId": "task2",
        "success": false,
        "message": "Problem already exists in queue"
      },
      {
        "userId": "user3",
        "taskId": "task3",
        "success": true,
        "message": "Problem uploaded successfully"
      }
    ],
    "uploadIP": "127.0.0.1"
  }
}
```

### Search Problems

**GET** `/api/problem/search?keyword=search_term&limit=10`

Search problems using Elasticsearch. Returns results with highlights.

Uses dynamic fuzziness and score thresholds based on keyword length:
- Short keywords (1-3 chars): strict matching, high score threshold (20.0)
- Medium keywords (4-10 chars): moderate matching, medium threshold (15.0-10.0)
- Long keywords (11+ chars): fuzzy matching, low threshold (5.0-2.0)

**Query Parameters:**
- `keyword` (required): Search term
- `limit` (required): Number of results (5-20)

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 42,
    "results": [
      {
        "id": "problem_id",
        "mongo_id": "mongo_id",
        "user_review": "review text",
        "review_pics": ["url1", "url2"],
        "timestamp": 1234567890,
        "problem_type": 1,
        "answer": 1,
        "ratio_1": 75.5,
        "ratio_2": 24.5,
        "uploader": "uploader_id",
        "taskId": "task_id",
        "userId": "user_id",
        "created_at": 1234567890,
        "_score": 1.5,
        "_highlight": {
          "searchable_content": ["<mark>highlighted</mark> text"],
          "user_review": ["<mark>review</mark> content"]
        },
        "replies": [...],
        "appeals": [...],
        "orders": [...],
        "order_detail": {...},
        "comments": [...]
      }
    ]
  }
}
```

### Search by Elasticsearch ID

**GET** `/api/problem/by-esid/:id`

Get a specific problem by its Elasticsearch document ID.

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "problem_id",
    "mongo_id": "mongo_id",
    "stars": 5,
    "user_review": "review text",
    "problem_type": 1,
    "_score": 0,
    ...
  }
}
```

### Search by MongoDB ID

**GET** `/api/problem/by-mongoid/:id`

Get a specific problem by its MongoDB ID.

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "problem_id",
    "mongo_id": "requested_mongo_id",
    "stars": 5,
    "user_review": "review text",
    "problem_type": 1,
    "_score": 1.0,
    ...
  }
}
```

---

## User Detail (Protected)

### Get User Avatar

**GET** `/api/user_detail/avatar?userId=xxx&token=xxx`

Get user's avatar image. Returns the cached image file directly. Caches avatars in `cache/img/avatar_{userId}.{ext}`.

**Note:** This endpoint accepts the auth token via query parameter (instead of Authorization header) because it's used in `<img src="">` tags which cannot send custom headers.

**Query Parameters:**
- `userId` (required): The user ID to look up
- `token` (required): JWT authentication token

**Response:**
- Returns image file directly (Content-Type: image/jpeg, image/png, etc.)
- Returns 401 if token is invalid or missing
- Returns 404 if user has no non-anonymous comments or no avatar

### Get User Info

**GET** `/api/user_detail/user_info?userId=xxx`

Get user information including username, total likes, and total replies.

**Query Parameters:**
- `userId` (required): The user ID to look up

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "userName": "用户昵称",
    "likes": 12345,
    "replies": 678
  }
}
```

**Error Response (404):**
```json
{
  "code": 404,
  "message": "No non-anonymous comment found for this user"
}
```

### Get User Comments

**GET** `/api/user_detail/comments?userId=xxx&page=1&limit=10`

Get paginated comments for a user, sorted by approveCount (desc) and createTime (desc).

**Query Parameters:**
- `userId` (required): The user ID to look up
- `page` (optional, default: 1): Page number
- `limit` (optional, default: 10, max: 100): Number of comments per page

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "comments": [
      {
        "id": "mongo_object_id_hex",
        "problemId": "problem_mongo_id",
        "commentId": "1754799712775200769",
        "userId": "3417484203",
        "userName": "用户昵称",
        "userPic": "https://...",
        "createTime": 1707211828,
        "content": "评论内容...",
        "approveCount": 1489,
        "replyTotal": 34,
        "isAnonymous": false,
        "voteOperate": "DOWN",
        "choice": 1,
        "images": ["https://..."],
        "audios": [
          {
            "url": "https://...",
            "duration": 2,
            "audioText": "语音转文字内容"
          }
        ]
      }
    ],
    "total": 42,
    "page": 1,
    "limit": 10,
    "totalPages": 5
  }
}
```

**Choice Values:**
- `1`: Support User (适合展示) - User voted DOWN (or UP if reversed for DAOZONG_JIAOYI/IPR)
- `2`: Support Merchant (不适合展示) - User voted UP (or DOWN if reversed for DAOZONG_JIAOYI/IPR)

### Get Rankings

**GET** `/api/user_detail/rankings?page=1`

Get users ranked by total likes (approveCount) across all their comments, paginated at 100 users per page.

**Query Parameters:**
- `page` (optional, default `1`): 1-indexed page number. Out-of-range pages are clamped to the last available page.

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "rankings": [
      {
        "userId": "3417484203",
        "userName": "用户昵称",
        "likes": 15890,
        "commentCount": 234,
        "rank": 1
      },
      {
        "userId": "1234567890",
        "userName": "另一个用户",
        "likes": 12345,
        "commentCount": 156,
        "rank": 2
      }
    ],
    "total": 4523,
    "page": 1,
    "pageSize": 100,
    "totalPages": 46
  }
}
```

**Notes:**
- Only includes users with non-anonymous comments (to get userName)
- Rankings are precomputed by `setup/comment_sync.py` (`sync_user_rankings`) into the `user_rankings` collection each time the comment sync job runs, rather than aggregated from the full `comments` collection on every request. Ranking freshness therefore matches the sync schedule.
- `rank` is each user's 1-based position in the full leaderboard, so it stays correct across pages (not just the item's position within the current page).

### Search Users

**GET** `/api/user_detail/search_users?keyword=xxx&limit=10`

Finds distinct users whose nickname (as it appears on any of their comments) contains the given keyword, case-insensitive. Intended for a typeahead/autocomplete search box (e.g. above the rankings list) that resolves a nickname to a `userId` for navigation.

**Query Parameters:**
- `keyword` (required): Substring to match against `userName`. Matched literally (regex metacharacters are escaped), case-insensitive.
- `limit` (optional, default `10`, max `20`): Maximum number of distinct users to return.

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "users": [
      { "userId": "3417484203", "userName": "用户昵称A" },
      { "userId": "1234567890", "userName": "用户昵称AB" }
    ]
  }
}
```

**Notes:**
- Only considers non-anonymous comments (same restriction as Rankings, since anonymous comments don't carry a usable `userName`).
- A user may have posted under different nicknames over time; results are de-duplicated by `userId`, keeping the first nickname encountered for that user.
- No dedicated text index is used; matching is a case-insensitive substring `$regex` over the `userName` field (which has a plain index for filtering). Intended for interactive, short keyword queries rather than full-text search.

### Get AI User Summary

**GET** `/api/user_detail/ai_summary?userId=xxx`

Returns the cached AI-generated "犀利点评 + 用户画像" for a user, if one exists, without triggering generation.

**Query Parameters:**
- `userId` (required): The user ID to look up

**Response:**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "ready",
    "result": {
      "roast": "...",
      "profile": {
        "summary": "...",
        "expressionStyle": ["..."],
        "opinionTendency": ["..."],
        "interactionPattern": ["..."],
        "genderGuess": { "value": "无法从文本判断", "confidence": "low", "disclaimer": "非事实、不可用于判断真实身份" },
        "mbtiGuess": { "value": "无法从文本判断", "confidence": "low", "disclaimer": "非心理测量、不可用于判断真实人格" }
      },
      "evidence": [
        { "claim": "...", "evidenceIds": ["c1", "c5"], "reason": "..." }
      ],
      "limitations": ["..."]
    },
    "provider": "deepseek",
    "model": "deepseek-v4-flash",
    "promptVersion": "user_profile_summary_v1",
    "generatedAt": 1752745200,
    "expiresAt": 1753350000,
    "stale": false
  }
}
```

`status` is one of `none` (never generated), `ready` (has a result), or `failed` (last attempt errored; `lastError` is included). `stale` is `true` once `generatedAt` is more than 7 days old — the frontend uses this to show a refresh button instead of re-fetching automatically.

### Generate AI User Summary

**POST** `/api/user_detail/ai_summary?userId=xxx`

Generates a new AI summary if there is no cached result or the cached result is older than 7 days; otherwise returns the existing cache unchanged. Generation calls DeepSeek first, then falls back to OpenRouter if DeepSeek fails, times out, or returns an invalid structured response. Only up to 300 of the user's highest-liked comments (redacted: no username/userId/avatars/media URLs) are sent to the model. Samples are added highest-liked first and capped by a total input-length budget rather than just the 300-item count, so a user with unusually long/numerous comments may end up with fewer than 300 samples — the model always sees the most-liked ones first.

**Query Parameters:**
- `userId` (required): The user ID to summarize

**Response:** Same shape as the GET endpoint above.

**Error Response (409):** returned if a generation for this user is already in progress (e.g. a double click); retry after a few seconds.
```json
{
  "code": 409,
  "message": "AI summary is already being generated for this user, please retry shortly"
}
```
