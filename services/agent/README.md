# LogPulse file tail agent

Small Go binary that tails a log file and forwards new lines to the ingest service as JSON batches.

## Usage

```bash
go run ./services/agent --file /var/log/myapp.log --ingest-url http://localhost:8080/v1/logs --service myapp
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--file` | (required) | Path to the log file |
| `--ingest-url` | `http://localhost:8080/v1/logs` | Ingest batch endpoint |
| `--service` | `file-agent` | Default `service` field |
| `--batch-size` | `20` | Max lines per POST |
| `--flush-interval` | `1s` | Max delay before flushing a partial batch |

The agent starts at **end of file** (like `tail -f`) so existing history is not replayed on restart.

## Docker Compose

Run locally against the stack:

```bash
docker compose up -d ingest
go run ./services/agent --file /tmp/demo.log --ingest-url http://localhost:8080/v1/logs
```
