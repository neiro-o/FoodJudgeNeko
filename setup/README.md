# Setup Script

Python script to initialize MongoDB database and create initial admin user.

## Prerequisites

- Python 3.7 or higher
- MongoDB running and accessible
- Required Python packages (install with `pip install -r requirements.txt`)

## Installation

```bash
cd setup
pip install -r requirements.txt
```

## Usage

```bash
python init.py
```

The script will:
1. Load configuration from `../config.yml`
2. Connect to MongoDB
3. Create indexes for `accounts` and `invitations` collections
4. Prompt you to enter:
   - Admin username
   - Admin email
   - Admin password (twice for confirmation)
5. Create the initial admin user with `is_admin: true`

## Export vanished high-liked comments

`fetch_vanished_comments.py` finds a user's comments with `approveCount > 35`
whose `commentId` no longer appears under the corresponding problem's
`comment[].pageContent[]`, then exports them as CSV. `createTime` is written in
Singapore Time (SGT, UTC+08:00). For each result, the script also looks up the
Elasticsearch document by `mongo_id`: it exports `user_review`, falling back to
`appeals[0].content` when `user_review` is empty, and converts the ES `timestamp`
to SGT.

```bash
python fetch_vanished_comments.py 123456789
python fetch_vanished_comments.py 123456789 -o /path/to/result.csv
```

The default output is `<user_id>.csv` in the current
directory. Comments with an invalid `problemId`, or whose entire problem no
longer exists, are skipped rather than counted as vanished comments. The final
columns are `problemId`, `user_review`, `timestamp`, `commentId`, `content`,
`likes`, `createTime`, and `url`; the URL is assembled from the problem's
`userId` and `taskId`.

## Notes

- The script checks if a user with the same username or email already exists
- Passwords are hashed using bcrypt before storage
- The admin user will have `is_admin` set to `true`
