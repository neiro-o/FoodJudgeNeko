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

