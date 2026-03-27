# CLAUDE.md

## What is cfntop?

A live TUI monitor for AWS CloudFormation stacks. Displays stacks sorted by last update with active deployments on top. Expand a stack to see resources, their current status, ECS deployment details, and recent event history for errored resources.

**Core principles:**
- cfntop is read-only. It never modifies stacks.
- cfntop requires AWS read-only CloudFormation access.

## Usage

```
cfntop -r <region>          # Monitor stacks in a region
cfntop -p <profile>         # Use a specific AWS profile
cfntop -n 10                # Custom poll interval (seconds, default: 5)
cfntop --absolute-time      # Show absolute timestamps instead of relative
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `enter` / `space` | Expand / collapse stack |
| `r` | Force refresh |
| `q` / `ctrl+c` | Quit |

## Build & Test

```bash
go build -o cfntop ./cmd/cfntop    # Build
go test -race ./...                 # Run tests
go vet ./...                        # Vet
./watchexec.sh -r <region>          # Dev mode with hot reload (requires watchexec)
```

## Project Structure

- `cmd/cfntop/` - Entrypoint with version ldflags
- `internal/aws/` - AWS SDK v2 CloudFormation client (interface-based for testing)
- `internal/model/` - Domain types (StackSummary, StackList, Resource, StackEvent, StackEvents)
- `internal/tui/` - Bubbletea TUI
  - `app.go` - Model, View, Update loop
  - `poller.go` - Stack/resource fetching, deploy boundary detection, deleted resource tracking
  - `ecs.go` - ECS service deployment + failed task details
  - `format.go` - Resource type shortening, humanized times
  - `styles.go` - Lipgloss styles, status color coding

## Conventions

- Conventional commits (`feat:`, `fix:`, `chore:`)
- release-please manages versioning and changelog
- GoReleaser handles cross-compilation (chained off release-please, not tag push)
- Module path: `github.com/awesome-foundation/cfntop`
