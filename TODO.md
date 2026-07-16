# Finance Tracker Backend Todo

## Release 1 Backend Tasks

- [x] Implement quota update/archive
- [x] Implement category delete/bulk recategorization
- [x] Add counterparty loan ledger support
- [x] Add funding remediation flow endpoint
- [x] Add dashboard/report endpoints

## Frontend Tasks

- [x] Add UI shell and navigation
- [x] Wire UI forms to backend APIs
- [x] Add schedule actions to the dashboard
- [x] Add loan ledger dashboard widget

## Status

All backend requirements from Release 1 are implemented.

### Notes

- `main.go` exposes all required API routes, including quota breakdowns and category totals.
- `internal/domain` contains quota, category, transaction, schedule, and loan ledger logic.
- `go test ./...` passes successfully.
