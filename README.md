# liftoff-export-cli

Export and analyze your workout data from the [Liftoff](https://getgymbros.com) fitness app. A command-line tool to back up gym sessions, track bodyweight trends, and view exercise statistics — all from your terminal.

[![Latest Release](https://img.shields.io/github/v/release/quantcli/liftoff-export-cli)](https://github.com/quantcli/liftoff-export-cli/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/quantcli/liftoff-export-cli)](go.mod)
![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)

## Features

- **Workout export** — list, search, and display workouts in [fitdown](https://github.com/datavis-tech/fitdown) or JSON format
- **Exercise filtering** — filter by exercise name with word-prefix matching (e.g. `--exercise bench`)
- **Date filtering** — relative (`30d`, `4w`, `6m`, `1y`) or absolute (`2025-01-01`)
- **Exercise statistics** — per-exercise PR tracking, volume summaries, and monthly progress bar charts
- **Bodyweight tracking** — trend analysis with ASCII charts, plateau detection, and rate-of-change
- **Multi-platform** — pre-built binaries for macOS (Intel + Apple Silicon), Linux, and Windows
- **JSON output** — pipe workout data to jq, scripts, or other tools with `--json`

## Quick Start

```sh
# Install with Homebrew
brew tap quantcli/tap
brew install liftoff-export

# Log in and list recent workouts
liftoff-export auth login
liftoff-export workouts list --since 7d
```

## Install

**Homebrew (macOS / Linux):**
```sh
brew tap quantcli/tap
brew install liftoff-export
```

Or download a pre-built binary from the [releases page](https://github.com/quantcli/liftoff-export-cli/releases/latest):

**macOS (Apple Silicon):**
```sh
curl -Lo /tmp/liftoff-export.zip https://github.com/quantcli/liftoff-export-cli/releases/latest/download/liftoff-export_darwin_arm64.zip
unzip -jo /tmp/liftoff-export.zip -d ~/bin && rm /tmp/liftoff-export.zip
chmod +x ~/bin/liftoff-export
```

**macOS (Intel):**
```sh
curl -Lo /tmp/liftoff-export.zip https://github.com/quantcli/liftoff-export-cli/releases/latest/download/liftoff-export_darwin_amd64.zip
unzip -jo /tmp/liftoff-export.zip -d ~/bin && rm /tmp/liftoff-export.zip
chmod +x ~/bin/liftoff-export
```

**Linux (amd64):**
```sh
curl -Lo /tmp/liftoff-export.zip https://github.com/quantcli/liftoff-export-cli/releases/latest/download/liftoff-export_linux_amd64.zip
unzip -jo /tmp/liftoff-export.zip -d ~/bin && rm /tmp/liftoff-export.zip
chmod +x ~/bin/liftoff-export
```

**Windows (amd64):**

Download `liftoff-export_windows_amd64.zip` from the [releases page](https://github.com/quantcli/liftoff-export-cli/releases/latest), extract it, and add the directory to your PATH.

Make sure `~/bin` is in your `PATH`. If not, add this to your `~/.zshrc` or `~/.bashrc`:
```sh
export PATH="$HOME/bin:$PATH"
```

## Usage

### Auth

```sh
liftoff-export auth login      # Log in to Liftoff
liftoff-export auth logout     # Remove stored auth tokens
liftoff-export auth refresh    # Manually refresh the access token
```

### Workouts

```sh
liftoff-export workouts list                       # List workouts in fitdown format
liftoff-export workouts list --json                # Output as JSON
liftoff-export workouts list --since 30d           # Filter by relative date (30d, 4w, 6m, 1y)
liftoff-export workouts list --since 2025-01-01    # Filter by absolute date
liftoff-export workouts list --exercise bench      # Filter to matching exercises
liftoff-export workouts show 2025-03-08            # Show workout(s) for a date
liftoff-export workouts show today                 # Show today's workout
liftoff-export workouts show yesterday             # Show yesterday's workout
```

### Workout Stats

```sh
liftoff-export workouts stats                      # Per-exercise summaries with monthly graphs
liftoff-export workouts stats --detail             # Per-session breakdown
liftoff-export workouts stats --exercise curl      # Filter to matching exercises
liftoff-export workouts stats --since 6m           # Filter by date
liftoff-export workouts stats --json               # Output as JSON
```

### Bodyweights

```sh
liftoff-export bodyweights list                    # List recorded bodyweights
liftoff-export bodyweights list --since 6m         # Filter by date
liftoff-export bodyweights stats                   # Stats with monthly graph and trends
liftoff-export bodyweights stats --since 2025-01-01
```

## Output Format

Workouts are printed in [fitdown](https://github.com/datavis-tech/fitdown) format by default:

```
Workout January 30, 2025

Machine Tricep Extension
12@110
2x6@125

Assisted Pull Ups
3@-100

Scapular Pull Ups
10@+0

Walking
1.00mi 18:00
```

## About Liftoff

[Liftoff](https://getgymbros.com) is a fitness tracking app for logging gym workouts, bodyweight, and exercise progress. This CLI is an unofficial tool for exporting and analyzing your own data.
