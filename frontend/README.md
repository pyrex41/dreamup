# DreamUp QA Agent - Frontend

A web-based dashboard for managing and viewing QA test results, built with Elm and Vite.

## Features

- ✅ Elm 0.19.1 with Browser.application
- ✅ Client-side routing with URL parsing
- ✅ CORS-enabled HTTP helpers for API communication
- ✅ Vite build tool with hot module replacement
- ✅ Production-ready build optimization

## Project Structure

```
frontend/
├── src/
│   └── Main.elm           # Main application entry point
├── index.html             # HTML template with embedded styles
├── vite.config.js         # Vite configuration
├── elm.json               # Elm package dependencies
├── package.json           # npm dependencies and scripts
└── README.md              # This file
```

## Prerequisites

- Node.js 20.x or higher
- npm 10.x or higher
- Elm 0.19.1

## Installation

```bash
# Install npm dependencies
npm install
```

## Development

```bash
# Start development server on http://localhost:3000
npm run dev
```

The dev server includes:
- Hot module replacement
- CORS enabled
- Auto browser opening

## Building for Production

```bash
# Build optimized production bundle
npm run build
```

Output will be in the `dist/` directory.

## Preview Production Build

```bash
# Preview the production build locally
npm run preview
```

## Routes

The application supports the following routes:

- `/` - Home page with quick actions
- `/submit` - Test submission interface
- `/test/:id` - Test execution status tracking
- `/report/:id` - Report display
- `/history` - Test history and search

## API Configuration

The API base URL is currently set to `http://localhost:8080/api`. Update in `src/Main.elm` (line 44):

```elm
apiBaseUrl = "http://localhost:8080/api"  -- Update with actual API URL
```

## Dependencies

### Elm Packages

- `elm/browser` - Browser.application
- `elm/core` - Core Elm functionality
- `elm/html` - HTML rendering
- `elm/http` - HTTP requests
- `elm/json` - JSON encoding/decoding
- `elm/time` - Time handling
- `elm/url` - URL parsing
- `elm-community/list-extra` - Extended list utilities
- `justinmimbs/time-extra` - Extended time utilities

### npm Packages

- `vite` - Build tool
- `vite-plugin-elm` - Elm integration for Vite

## HTTP Helpers

The application includes CORS-ready HTTP helpers:

```elm
-- GET request with CORS headers
getWithCors : String -> Decoder a -> (Result Http.Error a -> Msg) -> Cmd Msg

-- POST request with CORS headers
postWithCors : String -> Value -> Decoder a -> (Result Http.Error a -> Msg) -> Cmd Msg
```

## Next Steps

1. Implement test submission form (Task 2)
2. Add test execution status tracking (Task 3)
3. Create report display component (Task 4)
4. Add screenshot viewer (Task 5)
5. Implement console log viewer (Task 6)
6. Add test history and search (Task 7)
7. Polish UI/UX and deploy (Task 8)

## License

ISC
