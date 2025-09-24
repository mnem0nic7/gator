# gator

`gator` is a simple CLI RSS aggregation tool written in Go. It lets you:

- Register/login users (stored in Postgres)
- Add and follow RSS feeds
- Continuously aggregate feeds on an interval (`agg <duration>`)
- Store feed posts in Postgres
- Browse recent posts from the feeds you follow (`browse [limit]`)

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
./gator register alice          # create user
./gator login alice             # switch current user
./gator addfeed hn https://hnrss.org/newest  # add and auto-follow feed
./gator follow https://wagslane.dev/index.xml
./gator following               # list followed feeds
./gator agg 1m                  # start aggregator (Ctrl+C to stop)
./gator browse 5                # show 5 most recent posts from followed feeds
```

If you omit the browse limit, it defaults to 2.

## Development

Regenerate `sqlc` code after changing queries:

```bash
sqlc generate
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

## Notes

- Aggregator currently fetches one feed per interval in fair rotation.
- Duplicate posts are ignored based on URL uniqueness.
- More features (tagging, read/unread) could be added later.

Enjoy!
