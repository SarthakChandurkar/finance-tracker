# Finance Tracker

A local-first personal finance tracker built in Go. The backend implements the domain rules for transactions, quotas, categories, and scheduled monthly operations, with JSON file persistence and simple HTTP APIs for import/export.

## Features

- Single transaction schema for all financial events
- Monthly and Global quota handling with linked quota pairs
- Balance validation with eligible funding option reporting
- Scheduled month-start allocation and month-end sweep logic
- Local JSON storage with atomic save semantics
- Simple JSON API for transactions, quotas, categories, import/export, and schedule operations

## Getting Started

### Requirements

- Go 1.26+

### Run

```bash
cd /home/cdot/Desktop/Sarthak/finance-tracker
go run main.go
```

Open `http://localhost:8080/` in your browser to use the built-in UI.

The server listens on `:8080` by default and uses `data.json` in the current directory.

### Configuration

- `DATA_FILE` — path for the JSON datastore (default `data.json`)
- `ADDR` — HTTP listen address (default `:8080`)

## API Endpoints

- `GET /api/status`
- `GET /api/transactions`
- `POST /api/transactions`
- `PUT /api/transactions/{id}`
- `DELETE /api/transactions/{id}`
- `GET /api/quotas`
- `POST /api/quotas`
- `GET /api/categories`
- `POST /api/categories`
- `GET /api/export`
- `POST /api/import`
- `POST /api/schedule/start`
- `POST /api/schedule/end`
- `GET /api/funding-options?quota_id={id}&amount={value}`

## Notes

- Import replaces the current dataset completely.
- The JSON store initializes automatically with the mandatory `Savings` quota and `Settled` category on first run.
