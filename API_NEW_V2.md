# New API Reference (v2)

Brief docs for APIs added in this session.

## 1. Random Image

Picks a random comment from `mtv2.comments` whose `images` field is a non-empty array, takes its first image (`images[0]`), and downloads/serves it using the same cache-then-serve flow as the existing image proxy (`GET /api/media/image`).

- **Method/Path**: `GET /api/media/random_image`
- **Auth**: None (public, same as other `/media/*` endpoints)
- **Query params**: none
- **Response** (`200`): raw image bytes (`Content-Type` set from cached file), served from `cache/img/` after downloading if not already cached.
- **Errors**:
  - `404` if no comment with a non-empty `images` array exists.
  - `500` if the image's host is not in the allowed domain list (`meituan.com` / `meituan.net` / `sankuai.com`), or if the download fails.

Implementation: `backend/handlers/media.go` (`LoadRandomImage`).

## 2. Points Leaderboard

Paginated leaderboard combining `mtv2.accounts` (all accounts) left-joined with each account's current-week entry in `mtv2.weekly_scores` (`weekly_scores.userId == accounts._id`, filtered to the current ISO year/week in Singapore Time). Accounts with no weekly-score entry for the current week are included with `score: null`.

Sort order: `score` descending (accounts with no score sort last), then `points` descending.

- **Method/Path**: `GET /api/points/leaderboard`
- **Auth**: Bearer token, any authenticated account (no admin check)
- **Query params** (both optional):
  - `page` (default `1`)
  - `limit` (default `50`, max `100`)

Example: `GET /api/points/leaderboard?page=1&limit=50`

- **Response** (`200`):

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "year": 2026,
    "weekId": 32,
    "rankings": [
      { "id": "665f1a2b3c4d5e6f7a8b9c0d", "username": "someuser", "points": 120, "score": 87.5 },
      { "id": "665f1a2b3c4d5e6f7a8b9c0e", "username": "otheruser", "points": 30, "score": null }
    ],
    "total": 452,
    "page": 1,
    "limit": 50,
    "totalPages": 10
  }
}
```

Implementation: `backend/handlers/account_admin.go` (`GetPointsLeaderboard`).

## 3. Points/Leaderboard Frontend Page

- **Route**: `/points`
- **Auth**: requires login (redirects to `/login` if unauthenticated), same guard pattern as `/app/user/page.tsx`.
- Fetches `GET /api/points/leaderboard` via `pointsAPI.getLeaderboard(page, limit)` (`frontend/lib/api.ts`).
- Paginated, default page size **50**, page controlled via `?page=` query param (same pagination UI/behavior as `/user_stats`).
- Added to `Navbar`'s title dropdown as **"分榜"** (zh) / **"Leaderboards"** (en) — translation keys `nav.points`, `points.*`, `pageTitle.points` in `frontend/contexts/LanguageContext.tsx`.

Implementation: `frontend/app/points/page.tsx`.
