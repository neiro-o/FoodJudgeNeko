# New API Reference

This file tracks new APIs added to the backend during this session, along with brief usage instructions.

## Admin Account Management

Both endpoints require authentication (`Authorization: Bearer <token>`) and the requesting account must have `is_admin = true` (checked via `utils.IsAdmin`). Otherwise a `401 Unauthorized` response is returned.

### 1. Create Account

- **Method/Path**: `POST /api/admin/accounts/create`
- **Auth**: Bearer token, admin only
- **Body**:

```json
{
  "username": "newuser",
  "email": "newuser@example.com"
}
```

- `id` is **not** accepted as input — it is always generated server-side (`primitive.NewObjectID()`) and returned in the response.
- `username` and `email` must not already be in use — checked separately, returning `409 Conflict` with `"Username already exists"` or `"Email already exists"` respectively (whichever conflicts first).
- The created account always has `points = 0` and `is_admin = false`.
- A random 16-character initial password (`utils.GenerateRandomPassword`) is generated and stored (bcrypt-hashed) automatically — no password is accepted as input.

- **Response** (`200`):

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "message": "Account created successfully",
    "user": {
      "id": "665f1a2b3c4d5e6f7a8b9c0d",
      "username": "newuser",
      "email": "newuser@example.com",
      "points": 0,
      "is_admin": false
    },
    "initial_password": "aB3$fGh7Kp9RmN2s"
  }
}
```

- **Important**: `initial_password` is returned in plaintext **only in this response**, at creation time. It is never stored in plaintext and cannot be retrieved again afterwards — only its bcrypt hash is kept in `accounts.password`. Make sure to relay it to the account owner immediately (e.g. so they can log in and change it via `POST /api/change-password`).

### 2. Query Account (email + points)

- **Method/Path**: `POST /api/admin/accounts/query`
- **Auth**: Bearer token, admin only
- **Body**: Provide exactly one of `id`, `username`, or `email`.

```json
{ "username": "someuser" }
```

or

```json
{ "email": "someuser@example.com" }
```

or

```json
{ "id": "665f1a2b3c4d5e6f7a8b9c0d" }
```

- **Response** (`200`):

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "665f1a2b3c4d5e6f7a8b9c0d",
    "username": "someuser",
    "email": "someuser@example.com",
    "points": 120
  }
}
```

- Returns `400` if zero or more than one of `id`/`username`/`email` is provided, or if `id` is not a valid ObjectID.
- Returns `404` if no matching account is found.

## Weekly Score Ranking

Data source: `mtv2.weekly_scores` collection, joined with `accounts` on `weekly_scores.userId == accounts._id`.

Collection schema (created by `setup/mongodb_init.py`):

```js
db.createCollection("weekly_scores", {
  validator: {
    $jsonSchema: {
      bsonType: "object",
      required: ["userId", "year", "weekId", "score", "createdAt", "updatedAt"],
      properties: {
        userId: { bsonType: "objectId", description: "必须对应 accounts._id" },
        year: { bsonType: "int", minimum: 2000 },
        weekId: { bsonType: "int", minimum: 1, maximum: 53 },
        score: { bsonType: ["int", "long", "double", "decimal"], minimum: 0 },
        createdAt: { bsonType: "date" },
        updatedAt: { bsonType: "date" }
      }
    }
  },
  validationLevel: "strict",
  validationAction: "error"
})
```

### Get Weekly Score Ranking

- **Method/Path**: `GET /api/weekly_scores/ranking`
- **Auth**: Bearer token, any authenticated account (no admin check)
- **Query params** (both optional, integers; default to the current ISO year/week if omitted):
  - `year`
  - `weekId`

- The default "current" ISO year/week (when `year`/`weekId` are omitted) is always computed in **Singapore Time (SGT, UTC+8)**, regardless of the server's local timezone. Week boundaries therefore roll over at Monday 00:00 SGT.

Example: `GET /api/weekly_scores/ranking?year=2026&weekId=31`

- **Response** (`200`), rankings sorted by `score` descending:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "year": 2026,
    "weekId": 31,
    "rankings": [
      {
        "userId": "665f1a2b3c4d5e6f7a8b9c0d",
        "username": "someuser",
        "email": "someuser@example.com",
        "points": 120,
        "score": 87.5
      }
    ]
  }
}
```

## Problem Upload: `accountId` Override + Points/Score Rewards

Applies to the existing `POST /api/problem/upload` and `POST /api/problem/upload-multiple` endpoints (`backend/handlers/problem.go`).

### New optional field: `accountId`

Each problem object (top-level body for `/upload`, each entry of `problems[]` for `/upload-multiple`) now accepts an optional `accountId` field:

```json
{
  "userId": "some-user-id",
  "taskId": "some-task-id",
  "accountId": "665f1a2b3c4d5e6f7a8b9c0d"
}
```

- `accountId` must be a valid MongoDB ObjectId hex string (an `accounts._id`).
- **Admin-only**: if `accountId` is set and the caller is not an admin (`utils.IsAdmin`), the request is rejected with `401 Unauthorized` (for `/upload-multiple`, if *any* entry sets `accountId` without admin rights, or any `accountId` is malformed, the *entire* request is rejected before anything is queued).
- If `accountId` is malformed (not a valid ObjectId), the request is rejected with `400 Bad Request`.
- If omitted, behavior is unchanged: the upload is attributed to the currently authenticated caller, exactly as before.
- When set (by an admin), the upload is attributed to that `accountId` instead of the caller — this is the account that receives the queue item's `accountId`, and the points/weekly-score rewards below.

### Points + weekly score rewards

After a problem is successfully pushed to the upload queue (i.e. `success: true` in the response):

1. The effective account (`accountId` override if provided and valid, otherwise the caller) has `points` incremented by 1 in `accounts`. If that account id doesn't exist in `accounts`, this (and step 2) is silently skipped — it does not fail the upload response.
2. That same account's current ISO year/week entry in `weekly_scores` has `score` incremented by 1 (upserted if the `{userId, year, weekId}` document doesn't exist yet, with `createdAt`/`updatedAt` set accordingly). The `year`/`weekId` used here are computed in **Singapore Time (SGT, UTC+8)**, not the server's local timezone — see `utils.ISOYearWeekSGT` (`backend/utils/sgt_time.go`).

For `/upload-multiple`, this happens once per successfully queued problem (so `n` successes → `+n` points and `+n` weekly score for that account, potentially split across multiple accounts if different `accountId` overrides are used per entry).
