Online multiplayer Qwixx dice game served as a TUI over SSH, written in Go.

## Build & Test

```bash
go build ./...
go test ./... -v -race
```

## Architecture

- All game state is in-memory (no database). Server restart clears all rooms.
- Each SSH connection gets its own Bubbletea program. Styles use the global lipgloss renderer forced to TrueColor in `main.go` since the process runs under systemd with no terminal.
- Concurrency is managed with `sync.RWMutex` and `sync.Map` -- never use channels for shared state.
- Lobby events and game events use Go channel-based fan-out broadcasting.

## Key Conventions

- Game rules must match physical Qwixx: 4 colored rows, triangular scoring, 5 marks to lock, 4 penalties or 2 locked rows ends the game.
- The `game/` package is pure logic with no TUI dependencies. Keep it that way.
- Deploy target is `GOOS=linux GOARCH=amd64`.
