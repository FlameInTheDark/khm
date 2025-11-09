# khm

A compact CLI and TUI tool for managing SSH known_hosts files.

## Installation

### Using Go install

```bash
go install github.com/FlameInTheDark/khm@latest
```

### From source

Clone and build:

```bash
git clone https://github.com/FlameInTheDark/khm.git
cd khm
go mod tidy
go build -o khm
```

### Commands

```bash
# Launch TUI for default known_hosts
khm

# Launch TUI explicitly
khm ui

# List hosts
khm list

# Create backup of known_hosts
khm backup

# Stash all keys for a host into stash_hosts
khm stash <host>

# Delete all keys for a host from known_hosts
khm delete <host>

# Show help
khm --help
```


### File selection

You can control which known_hosts file is used:

- `--file, -f`:
  - Custom known_hosts path for all commands.
- `SSH_KNOWN_HOSTS`:
  - Used when `--file` is not set.
- Default:
  - `~/.ssh/known_hosts` if nothing else is provided.


## TUI

Key bindings (known_hosts view):

- Up/Down: navigate hosts
- /: filter (live)
- Enter: toggle details for selected host
- d: delete selected host (with confirmation)
- s: stash selected host into stash_hosts
- t: toggle between known_hosts and stash_hosts view
- ?: toggle help
- q / Ctrl+C: quit

Stash behavior:

- Stash writes entries to `stash_hosts` (by default next to known_hosts).
- Stash view lets you inspect stashed hosts and restore them back.
- Restoring avoids adding duplicate keys to known_hosts.

## Examples

Basic:

```bash
# TUI for default known_hosts
khm

# TUI for a specific known_hosts
khm --file /path/to/known_hosts

# Stash a host
khm stash github.com

# Delete a host
khm delete github.com
```

## Development

```bash
git clone https://github.com/FlameInTheDark/khm.git
cd khm
go mod tidy
go test ./...
go build -o khm
```



## Structure

- `main.go`: CLI entry and commands
- `ui.go`: TUI launcher and helpers
- `internal/knownhosts`: known_hosts parsing and file operations
- `internal/ui`: TUI model and rendering

## Technical details

- Go 1.21+
- Bubble Tea, Bubbles, Lipgloss
- Minimal, focused dependency set

## License

MIT License (see LICENSE).