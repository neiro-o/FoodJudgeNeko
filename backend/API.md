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
        "choice": 1
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

**GET** `/api/user_detail/rankings`

Get top 100 users ranked by total likes (approveCount) across all their comments.

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
        "commentCount": 234
      },
      {
        "userId": "1234567890",
        "userName": "另一个用户",
        "likes": 12345,
        "commentCount": 156
      }
    ],
    "total": 100
  }
}
```

**Notes:**
- Only includes users with non-anonymous comments (to get userName)
- Returns up to 100 users sorted by total likes (descending)
