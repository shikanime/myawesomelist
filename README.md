# My Awesome List

A web application that displays curated projects from awesome-go, awesome-elixir, and awesome-javascript repositories with GitHub star counts.

## Features

- 🚀 **Modern Web UI**: Built with React Router and Tailwind CSS
- 📱 **Responsive Design**: Works on desktop and mobile devices
- 🔍 **Project Discovery**: Browse projects from Go, Elixir, and JavaScript ecosystems
- ⭐ **GitHub Stars**: Real-time star counts for each project
- 🏷️ **Categorized View**: Projects organized by categories
- 🔗 **Direct Links**: Click to visit project repositories

## Quick Start

### Prerequisites

- Go 1.25.0 or later
- PostgreSQL accessible locally or via DSN
- Internet connection (to fetch data from GitHub)
- Optional: GitHub Personal Access Token for higher API rate limits
- Node.js 18+ and npm (for the web app)

### Running the Application

1. **Clone the repository**:

```bash
git clone https://github.com/shikanime/myawesomelist.git
cd myawesomelist
```

2. **Install backend dependencies**:

```bash
go mod tidy
```

3. **Configure database DSN** (or rely on `PG*` envs):

```bash
export DSN="postgres://postgres@localhost:5432/postgres?sslmode=disable"
```

4. **Run database migrations**:

```bash
go run ./cmd/myawesomelist migrate up
```

5. **Run the server (CLI)**:

```bash
go run ./cmd/myawesomelist serve
```

- Override address via flag:
```bash
go run ./cmd/myawesomelist serve --addr 0.0.0.0:8080
```
- Address falls back to `HOST` and `PORT` envs:
  - `HOST` default: `localhost`
  - `PORT` default: `8080`

6. **Verify health**:
   ```
   http://localhost:8080/health
   ```

### Running the Web App (Frontend)

1. Open a new terminal and go to `www`:
```bash
cd www
```

2. Install dependencies:
```bash
npm install
```

3. If your API is not at `http://localhost:8080`, set:
```bash
export VITE_API_BASE_URL="http://localhost:8080"
```

4. Start the dev server:
```bash
npm run dev
```

- Vite will print a local URL (typically `http://localhost:5173`). Open it in your browser.

### GitHub API Rate Limits

The application fetches star counts from the GitHub API. To avoid rate limiting:

1. **Create a GitHub Personal Access Token**:
   - Go to GitHub Settings > Developer settings > Personal access tokens
   - Generate a new token with `public_repo` scope
   - No additional permissions needed for public repositories

2. **Set the token as an environment variable**:

```bash
export GITHUB_TOKEN=your_token_here
go run ./cmd/myawesomelist serve
```

- The server also supports `GH_TOKEN` as a fallback.

## Configuration

- `DSN`: Database source name (`driver://dataSourceName`). Example:
  - `postgres://postgres@localhost:5432/postgres?sslmode=disable`
- `PGUSER`/`PGDATABASE`/`PGHOST`/`PGPORT`: Used if `DSN` is not set.
- `HOST` and `PORT`: Bind address for the API server (defaults: `localhost:8080`).
- `GITHUB_TOKEN` or `GH_TOKEN`: Increases GitHub API rate limits.
- Frontend `VITE_API_BASE_URL`: Base URL for API calls (default `http://localhost:8080`).
