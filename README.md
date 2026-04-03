# Qwixx Online

A multiplayer implementation of the Qwixx dice game, playable entirely in the terminal over SSH.

```
ssh -p 2222 lucasacastro.cloud
```

## How It Works

The server runs a TUI (terminal user interface) over SSH. Players connect with any SSH client -- no installation required. The game supports 2-5 players per room with real-time multiplayer via room codes.

Built with Go using the [Charm](https://charm.sh) ecosystem (Wish, Bubbletea, Lipgloss).

## Development

```bash
# Run locally
go run .

# Run on a custom port
go run . --host 127.0.0.1 --port 3333

# Run tests
go test ./... -v -race
```

Connect locally with `ssh -p 2222 localhost`.

## Deployment

The game runs as a single binary on a VPS with systemd.

```bash
# First-time VPS setup (creates user, directories, firewall rule, systemd service)
ssh me@lucasacastro.cloud 'bash -s' < setup-vps.sh

# Deploy (cross-compiles and restarts service)
./deploy.sh me@lucasacastro.cloud
```

Pushes to `main` auto-deploy via GitHub Actions.

## Project Structure

```
main.go          Entry point, CLI flags
server/          SSH server, session management
lobby/           Room management, player events
game/            Game engine, rules, dice, scoring
tui/             Terminal UI screens and styles
```
