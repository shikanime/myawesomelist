# My Awesome List

A web application that displays curated projects from awesome-go, awesome-elixir, and awesome-javascript repositories with GitHub star counts.

## Features

- 🚀 **Modern Web UI**: Built with Templ and Tailwind CSS
- 📱 **Responsive Design**: Works on desktop and mobile devices
- 🔍 **Project Discovery**: Browse projects from Go, Elixir, and JavaScript ecosystems
- ⭐ **GitHub Stars**: Real-time star counts for each project
- 🏷️ **Categorized View**: Projects organized by categories
- 🔗 **Direct Links**: Click to visit project repositories

## Quick Start

### Prerequisites

- Go 1.25.0 or later
- Internet connection (to fetch data from GitHub)
- Optional: GitHub Personal Access Token for higher API rate limits

### Running the Application

1. **Clone the repository**:

   ```bash
   git clone <repository-url>
   cd myawesomelist
   ```

2. **Install dependencies**:

   ```bash
   go mod tidy
   ```

3. **Generate templ files** (if needed):

   ```bash
   templ generate
   ```

4. **Run the application**:

   ```bash
   go run ./app
   ```

5. **Open your browser** and navigate to:
   ```
   http://localhost:8080
   ```

### GitHub API Rate Limits

The application fetches star counts from the GitHub API. To avoid rate limiting:

1. **Create a GitHub Personal Access Token**:
   - Go to GitHub Settings > Developer settings > Personal access tokens
   - Generate a new token with `public_repo` scope
   - No additional permissions needed for public repositories

2. **Set the token as an environment variable**:

   ```bash
   export GITHUB_TOKEN=your_token_here
   go run ./app
   ```

   Or use the Makefile:

   ```bash
   GITHUB_TOKEN=your_token_here make dev-with-token
   ```

### Using Make (Optional)

If you have `make` installed, you can use the provided Makefile:

```bash
# Run in development mode
make dev

# Run with GitHub token
GITHUB_TOKEN=your_token make dev-with-token

# Build the application
make build

# Run the built application
make run

# Generate templ files
make templ

# Clean build artifacts
make clean
```

## Configuration

### Port Configuration

You can specify a custom port using either:

1. **Command line flag**:

   ```bash
   go run ./app -port=3000
   ```

2. **Environment variable**:
   ```bash
   PORT=3000 go run ./app
   ```

### GitHub API Configuration

- **GITHUB_TOKEN**: Personal access token for higher rate limits (optional)

## Architecture

The application is structured as follows:
