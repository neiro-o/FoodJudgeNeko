# MTV2 Frontend

Next.js frontend application with React, Tailwind CSS, and Webpack.

## Features

- User authentication (login/logout)
- Protected routes
- User center with:
  - Change password functionality
  - View points
  - Generate and manage invitation codes
- Responsive design for both PC and mobile

## Prerequisites

- Node.js 18+ and npm/yarn
- Backend API running on `http://localhost:8080` (or configure via environment variable)

## Installation

```bash
cd frontend
npm install
```

## Configuration

Create a `.env.local` file in the `frontend` directory to configure the API URL:

```env
NEXT_PUBLIC_API_URL=http://localhost:8080/api
```

If not set, it defaults to `http://localhost:8080/api`.

## Development

Run the development server:

```bash
npm run dev
```

Open [http://localhost:3000](http://localhost:3000) in your browser.

## Pages

- `/` - Home page (temporarily blank)
- `/login` - Login page
- `/problems` - Problems page (temporarily blank, requires authentication)
- `/user` - User center with tabs:
  - Change Password
  - Points & Invitations

## Local login preferences

The login page provides two independent checkboxes on one line:

- Remember password: immediately stores the entered username/password in this browser and fills them on the next visit; unchecking removes the saved copy.
- 120h auto login: stores login credentials and the session locally until a fixed deadline 120 hours after sign-in. Expired 24h API tokens are renewed using the existing login endpoint; renewals do not extend the deadline. Logout or rejected credentials disable auto login. Without this option the session is kept in sessionStorage.

These preferences are never sent to or stored on the server. Authentication still uses the existing login API. Older unlimited local sessions require one new sign-in after this update.

Protected pages and API 401 responses navigate to `/login?redirect=...`, preserving the original path, query and hash. Successful login returns to that same-site destination; invalid or external destinations fall back to `/problems`.

Run the authentication regression checks with `node --test scripts/test-auth.mjs`.

## Build

```bash
npm run build
npm start
```

## Tech Stack

- **Next.js 14** - React framework
- **TypeScript** - Type safety
- **Tailwind CSS** - Styling
- **Axios** - HTTP client
- **React Context** - State management for authentication

