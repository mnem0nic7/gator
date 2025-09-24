# gator

`gator` is a simple CLI RSS aggregation tool written in Go. It lets you:

- Register/login users (stored in Postgres)
- Add and follow RSS feeds
- Continuously aggregate feeds on an interval (`agg <duration>`), with an optional service wrapper that restarts the worker
- Store feed posts in Postgres (duplicates skipped by URL)
- Browse, sort, filter, and page through recent posts from the feeds you follow
- Fuzzy-search posts by title/description
- Bookmark posts for later
- Launch a lightweight terminal UI for browsing posts
- Experiment with a simple HTTP API façade

## Requirements

- Go 1.21+ installed
- PostgreSQL running locally (default connection used):
  `postgres://postgres:postgres@localhost:5433/gator?sslmode=disable`

You can adjust the DB URL via the config file.

## Install

```bash
go install ./...
# or build locally
go build -o gator .
```

Make sure your `GOBIN` (or `$GOPATH/bin`) is on your `PATH` if you used `go install`.

## Configuration

Config is stored at `~/.gatorconfig.json` and looks like:

```json
{
  "db_url": "postgres://postgres:postgres@localhost:5433/gator?sslmode=disable",
  "current_user_name": ""
}
```

It's created automatically on first `register` or `login` if missing.

## Database Setup

Migrations use `goose`. To run them manually:

```bash
GOOSE_DRIVER=postgres GOOSE_DBSTRING="postgres://postgres:postgres@localhost:5433/gator?sslmode=disable" \
  goose -dir sql/migrations up
```

## Common Commands

```bash
./gator register alice                      # create user
./gator login alice                         # switch current user
./gator addfeed hn https://hnrss.org/newest # add and auto-follow feed
./gator follow https://wagslane.dev/index.xml
./gator following                           # list followed feeds

# Aggregation
./gator agg 1m            # fetch feeds every minute (Ctrl+C to stop)
./gator aggservice 1m     # keep agg running; restarts automatically on crash

# Browsing & discovery
./gator browse 5 0 title asc            # limit, offset, sort field, sort order
./gator browse 10 0 published_at desc   # default ordering (limit defaults to 2)
./gator search boot                     # fuzzy-search titles/descriptions
./gator bookmark <post-uuid>            # bookmark a post you've discovered
./gator tui                             # open an interactive terminal UI

# API (experimental)
./gator api              # serve HTTP API on :8080 (Ctrl+C to stop)
```

Browsing arguments are optional; the defaults are `limit=2`, `offset=0`, `sort=published_at`, `order=desc`, and no feed filter.

**Need post IDs?** Run a SQL query (for example with `psql`) against the `posts` table or extend the CLI output to include IDs when needed.

## Development

Regenerate `sqlc` code after changing queries:

```bash
sqlc generate
```

### Local testing & smoke checks

```bash
go test ./...
go run . reset
go run . register tester
go run . login tester
go run . addfeed "Boot Dev" https://blog.boot.dev/index.xml
timeout 10s go run . agg 2s   # optional: fetch posts quickly, press Ctrl+C to stop if you skip timeout
go run . browse 5             # confirm posts are stored locally
```

## Testing

Simple parser tests can be run with:

```bash
go test ./...
```

## Pushing to GitHub

```bash
git init
git add .
git commit -m "Initial gator RSS aggregator"
git branch -M main
git remote add origin git@github.com:mnem0nic7/gator.git
git push -u origin main
```

When updating existing work, replace the commit message with something descriptive, for example `git commit -m "Add CLI search and bookmarking"`.

## Notes

- Aggregator currently fetches one feed per interval in fair rotation.
- Duplicate posts are ignored based on URL uniqueness.
- More features (tagging, read/unread) could be added later.

Enjoy!
