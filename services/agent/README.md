# LogPulse file tail agent (planned)

Stretch goal: a small Go binary that tails one or more log files, parses lines into LogPulse `Entry` JSON, and POSTs batches to the ingest service (`POST /v1/logs`).

Planned flags:

- `--ingest-url` — ingest base URL (default `http://localhost:8080`)
- `--file` — path to log file (repeatable)
- `--service` — default service name for unparsed lines
- `--batch-size` / `--flush-interval` — buffering controls

This keeps edge hosts able to forward logs without syslog or app SDK changes.
