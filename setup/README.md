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

## Notes

- The script checks if a user with the same username or email already exists
- Passwords are hashed using bcrypt before storage
- The admin user will have `is_admin` set to `true`

