# XDCC Web Interface

React frontend with Express backend for XDCC CLI.

## Project Structure

```
web/
├── server/           # Express backend
│   ├── src/
│   │   ├── index.ts       # Express server with API routes
│   │   └── XdccCli.ts     # XDCC CLI wrapper
│   └── tsconfig.json
├── client/           # React frontend (Vite)
│   ├── src/
│   │   ├── App.tsx
│   │   └── main.tsx
│   ├── index.html
│   ├── vite.config.ts
│   └── tsconfig.json
├── dist/             # Production build output
│   ├── server/       # Built Express server
│   └── public/       # Built React app
├── downloads/        # XDCC downloads directory
└── package.json
```

## Development

Run both client and server in development mode:

```bash
npm run dev
```

This starts:
- Express server on `http://localhost:3000` (API endpoints)
- Vite dev server on `http://localhost:5173` (React app with HMR)

The Vite dev server proxies `/api/*` requests to the Express server.

### Run separately

```bash
# Terminal 1 - Express server
npm run dev:server

# Terminal 2 - React dev server
npm run dev:client
```

## Production Build

Build both client and server:

```bash
npm run build
```

This creates:
- `dist/server/` - Compiled Express server
- `dist/public/` - Built React app (static files)

For reverse-proxied deployments, set the `VITE_BASENAME`
environment variable before building:

```bash
VITE_BASENAME=/xdcc/ npm run build
```

## Running in Production

```bash
npm start
```

The Express server serves:
- API routes at `/api/*`
- React app static files from `dist/public/`
- SPA fallback for client-side routing

## API Endpoints

- `GET /api/health` - Health check
- `GET /api/search?searchString=<query>` - Search for files
- `GET /api/download?url=<irc-url>` - Download file (Server-Sent Events)

## Environment Variables

- `XDCC_PATH` - Path to xdcc binary (default: `../bin/xdcc`)
- `XDCC_DOWNLOADS_PATH` - Download directory (default: `./downloads`)

