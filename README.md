# liftoff-export-cli

A command-line interface for the [Liftoff](https://getgymbros.com) fitness app.

## Install

Download the latest release for your platform from the [releases page](https://github.com/DTTerastar/liftoff-export-cli/releases/latest), unzip it, and place the binary in `~/bin`.

**macOS (Apple Silicon):**
```sh
curl -Lo /tmp/liftoff-export.zip https://github.com/DTTerastar/liftoff-export-cli/releases/latest/download/liftoff-export_darwin_arm64.zip
unzip -jo /tmp/liftoff-export.zip -d ~/bin && rm /tmp/liftoff-export.zip
chmod +x ~/bin/liftoff-export
```

**macOS (Intel):**
```sh
curl -Lo /tmp/liftoff-export.zip https://github.com/DTTerastar/liftoff-export-cli/releases/latest/download/liftoff-export_darwin_amd64.zip
unzip -jo /tmp/liftoff-export.zip -d ~/bin && rm /tmp/liftoff-export.zip
chmod +x ~/bin/liftoff-export
```

**Linux (amd64):**
```sh
curl -Lo /tmp/liftoff-export.zip https://github.com/DTTerastar/liftoff-export-cli/releases/latest/download/liftoff-export_linux_amd64.zip
unzip -jo /tmp/liftoff-export.zip -d ~/bin && rm /tmp/liftoff-export.zip
chmod +x ~/bin/liftoff-export
```

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
