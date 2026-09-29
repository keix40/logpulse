# Contributing to LogPulse

LogPulse uses **trunk-based development**: `main` is always deployable. Short-lived feature branches merge via pull request.

## Branch naming

| Prefix | Use |
|--------|-----|
| `feat/*` | New features or user-visible behavior |
| `fix/*` | Bug fixes |
| `chore/*` | Tooling, deps, CI |
| `docs/*` | Documentation only |

Cloud agent branches may use `cursor/*` when created by automation.

## Commits

Follow [Conventional Commits](https://www.conventionalcommits.org/):

- `feat(ingest): add syslog listener`
- `fix(alerter): respect cooldown after burst`
- `test(alertengine): cover window expiry`
- `chore(ci): pin Go 1.22`

Reference GitLab/GitHub issues in the subject or body when applicable (`#123`).

## Pull requests

- Keep PRs focused; prefer several small PRs over one large change.
- Ensure `go test ./...` (from repo root with `go work sync`) and `npm test` / `npm run lint` in `web/` pass.
- Update `README.md` when behavior, config, or APIs change.
- Do not commit secrets; use `.env.example` as the template.

## Local checks

```bash
go work sync
go test ./pkg/... ./services/...
cd web && npm install && npm test && npm run lint
docker compose up --build
```
