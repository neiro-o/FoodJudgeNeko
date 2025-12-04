# Go Backend API

A Go backend API with authentication and invitation code management using MongoDB.

## Features

- Account login/logout
- Change password
- Generate invitation codes
- Register with invitation code

## Prerequisites

- Go 1.21 or higher
- MongoDB running locally or accessible via connection string

## Setup

1. **Install dependencies:**
   ```bash
   cd backend
   go mod download
   ```

2. **Configure MongoDB:**
   Edit `../config.yml` (or `config.yml` in root) with your MongoDB connection details:
   ```yaml
   mongodb:
     connection_string: "mongodb://localhost:27017"
     database_name: "mtv2"
     collections:
       accounts: "accounts"
       invitations: "invitations"
   ```

3. **Initialize database indexes:**
   ```bash
   cd ../setup
   go mod download
   go run init.go
   ```

4. **Run the server:**
   ```bash
   cd ../backend
   go run main.go
   ```

The server will start on port 8080 (or the port specified in `config.yml`).

## API Endpoints

### Public Endpoints

- `POST /api/login` - Login with username and password
  ```json
  {
    "username": "user123",
    "password": "password123"
  }
  ```

- `POST /api/register` - Register a new account with invitation code
  ```json
  {
    "username": "newuser",
    "password": "password123",
    "email": "user@example.com",
    "invite_code": "invitation-code-here"
  }
  ```

### Protected Endpoints (Require Bearer Token)

- `POST /api/logout` - Logout (client-side token removal)

- `POST /api/change-password` - Change password
  ```json
  {
    "old_password": "oldpass",
    "new_password": "newpass"
  }
  ```

- `POST /api/invitations/generate` - Generate invitation codes
  ```json
  {
    "count": 1
  }
  ```

- `GET /api/invitations` - List all invitations created by the authenticated user

## Authentication

All protected endpoints require a Bearer token in the Authorization header:
```
Authorization: Bearer <token>
```

The token is obtained from the login endpoint and is valid for 24 hours.

