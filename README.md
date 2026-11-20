# liftoff-export-cli

Export and analyze your workout data from the [Liftoff](https://getgymbros.com) fitness app. A command-line tool to back up gym sessions, track bodyweight trends, and view exercise statistics — all from your terminal.

[![Latest Release](https://img.shields.io/github/v/release/quantcli/liftoff-export-cli)](https://github.com/quantcli/liftoff-export-cli/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/quantcli/liftoff-export-cli)](go.mod)
![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)

## Features

- **Workout export** — list, search, and display workouts in [fitdown](https://github.com/datavis-tech/fitdown), JSON, or CSV format
- **Exercise filtering** — filter by exercise name with word-prefix matching (e.g. `--exercise bench`)
- **Date filtering** — relative (`30d`, `4w`, `6m`, `1y`) or absolute (`2025-01-01`)
- **Exercise statistics** — per-exercise PR tracking, volume summaries, and monthly progress bar charts
- **Bodyweight tracking** — trend analysis with ASCII charts, plateau detection, and rate-of-change
- **Multi-platform** — pre-built binaries for macOS (Intel + Apple Silicon), Linux, and Windows
- **JSON output** — pipe workout data to jq, scripts, or other tools with `--format json`
- **CSV output** — open workouts, stats, bodyweights, and routines in a spreadsheet with `--format csv`

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
liftoff-export auth import     # Save a refresh token captured from the app
liftoff-export auth logout     # Remove stored auth tokens
liftoff-export auth refresh    # Manually refresh the access token
```

**Google Sign-In accounts** have no password, so `auth login` can't be used. Log
in on the phone app with an HTTPS proxy (mitmproxy, Proxyman) running, copy the
`refreshToken` from the `user.signIn` response, and hand it to `auth import`:

```sh
liftoff-export auth import --refresh-token "$RT"   # verifies + saves
echo "$RT" | liftoff-export auth import            # same, token on stdin
```

For CI or containers, set `LIFTOFF_REFRESH_TOKEN` instead and skip the token file
entirely.

### Workouts

```sh
liftoff-export workouts list                       # List workouts in fitdown format
liftoff-export workouts list --format json         # Output as JSON
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
liftoff-export workouts stats --format json        # Output as JSON
```

### Bodyweights

```sh
liftoff-export bodyweights list                    # List recorded bodyweights
liftoff-export bodyweights list --since 6m         # Filter by date
liftoff-export bodyweights stats                   # Stats with monthly graph and trends
liftoff-export bodyweights stats --since 2025-01-01
```

### Routines

Routines are reusable workout templates saved in the Liftoff app (the upstream API calls them "presets"; the JSON output preserves that naming):

```sh
liftoff-export routines list                       # List all your saved routines (fitdown)
liftoff-export routines list --format json         # Full JSON for jq / agents
liftoff-export routines show Push                  # One routine by name (case-insensitive)
liftoff-export routines show cmkbk9ugu0eej3pv0oyd41x8c   # …or by id
```

Folder-organized routines are not rendered yet — file an issue if you need folder support.

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

### CSV

Every data subcommand also takes `--format csv` for spreadsheets, pandas, and the like:

```
$ liftoff-export bodyweights list --since 7d --format csv
date,weight
2026-06-08,175
2026-06-10,174.5
```

- One header row, always — an empty window prints the header alone.
- One flat row per data point, with the parent's columns repeated: per **set** for `workouts list` / `workouts show` / `routines list` / `routines show`, per **exercise session** for `workouts stats`, per **day** for `bodyweights list`, per **month** for `bodyweights stats`.
- Columns are the `--format json` keys in snake_case (`startedAt` → `started_at`). `input_one` / `input_two` are the raw set values; what they mean depends on `exercise_types` — see [Units and set values](#units-and-set-values).
- Timestamps are RFC3339 in your local zone; a missing value is an empty cell, never `0`.
- Standard RFC 4180 quoting, so notes containing commas, quotes, or line breaks survive a round trip.

## Units and set values

Set weights are **pounds**, and `inputOne` / `inputTwo` mean different things
depending on the exercise. If you consume `--format json` or `--format csv`
directly — in a script, a notebook, or an LLM agent — read this before you
aggregate anything. Both facts fail silently: you get plausible numbers that are
wrong, not an error.

### Set weights are canonical pounds

Liftoff stores a single weight unit. Kilogram input is converted to pounds
before the API returns it, and the original unit is not exposed anywhere in the
payload. This CLI applies no weight conversion of its own, so every exported
weight is pounds and there is no unit field to check.

Because the pound-per-kilogram factor does not divide evenly, kilogram-origin
values arrive with a long fractional tail. That tail is the only tell:

| exported `inputOne` (lb) | actual kg |
|---|---|
| 17.6369809748 | 8 |
| 22.0462262185 | 10 |
| 35.2739619496 | 16 |
| 39.6832071933 | 18 |
| 48.5016976807 | 22 |
| 110.2311310925 | 50 |
| 198.4160359665 | 90 |

Natively-entered pound values are clean increments by contrast — `230`, `177`,
`75`, `40`.

**Why this matters:** a 10 kg lateral raise exports as `22.0462262185` and so
compares as *lower* than a 28 lb set done on home dumbbells, even though it is
heavier. Trend lines, PR detection, and volume totals silently mix two
measurement systems. A 90 kg machine press (`198.4160359665`) likewise reads as
a new PR over a genuine 177 lb barbell best.

### Spotting kilogram-origin values

The fractional tail is detectable. This flags weighted sets whose `inputOne`
carries more than four decimal places and recovers the kilogram value:

```sh
liftoff-export workouts list --format json \
| jq -r '.[] | .startedAt[:10] as $date | .exerciseData[]
    | select(.exerciseTypes | IN("WR","AB","BR","WD"))
    | .exerciseName as $name | .setsData[].inputOne | tostring
    | select((split(".")[1] // "" | length) > 4)
    | "\($date)  \($name)  \(.) lb  = \((tonumber / 2.20462262185) * 100 | round / 100) kg"'
```

```text
2026-11-17  Seated Cable Chest Press  110.2311310925 lb  = 50 kg
2026-11-17  Seated Cable Chest Press  198.4160359665 lb  = 90 kg
2026-11-17  Dumbbell Lateral Raise  22.0462262185 lb  = 10 kg
2026-11-17  Kettlebell Bottoms Up  17.6369809748 lb  = 8 kg
```

A result that lands on a whole kilogram, as all four above do, is very likely
kilogram-origin. Treat this as a heuristic and not as provenance: the unit
origin genuinely is not in the data, so nothing here can distinguish a 90 kg
machine press from a dialed-in 198.4160359665 lb one. Rounding in the app can
also leave a short tail on a native pound entry, which is why the filter tests
for a *long* tail rather than for any fraction at all.

### `inputOne` / `inputTwo` depend on `exerciseTypes`

The two set fields are a generic pair, not weight and reps. `exerciseTypes` on
the parent exercise is the dispatch field, and it is exported — as the
`exerciseTypes` JSON key and the `exercise_types` CSV column — so the semantics
are recoverable. Six codes are live:

| `exerciseTypes` | `inputOne` | `inputTwo` |
|---|---|---|
| `WR` | weight (lb) | reps |
| `AB` | assistance (lb), subtracted from bodyweight | reps |
| `BR` | added load (lb), added to bodyweight | reps |
| `WD` | weight (lb) | distance (km) |
| `DD` | distance (km) | duration (seconds) |
| `ND` | unused | duration (seconds) |

Read `inputOne` as a weight without checking `exerciseTypes` and a one-hour
hike (`DD`, `inputOne` = `8.04672` km) becomes an 8 lb lift with 3600 reps.

The `fitdown` default output renders each code distinctly, so it is a quick way
to see which type an exercise is: `12@28` for `WR`, `3@-100` for `AB`, `6@+25`
for `BR`, `80lb 0.249mi` for `WD`, `5.00mi 60:00` for `DD`, and a bare `8:15`
for `ND`. Note that `fitdown` converts the `WD` and `DD` kilometer values to
miles for display; `--format json` and `--format csv` leave them in kilometers.

### What `workouts stats` already handles

`workouts stats` dispatches on `exerciseTypes` itself, so its own summaries do
not mix units. The risk above is to downstream consumers of raw `--format json`
and `--format csv` output, not to this CLI's own summaries:

- `WR`, `AB`, `BR` produce volume and best-weight, with `AB` and `BR` folding
  `bodyweight` into effective load (175 lb bodyweight with 100 lb assistance
  summarizes as `75x3`; with 25 lb added, as `200x6`).
- `DD` accumulates `duration` and `distance` only, and never contributes to
  volume or best-weight.
- `WD` and `ND` contribute a set count only, so neither pollutes volume or
  best-weight. The trade-off is that a `WD` exercise gets no summary lines and
  its monthly graph plots `0`; read its weight and distance from
  `workouts list` instead.
- In `stats --format json`, a `DD` session's `reps` field repeats the duration
  in seconds. Use `duration` instead; `reps` is not a rep count for `DD`.

## About Liftoff

[Liftoff](https://getgymbros.com) is a fitness tracking app for logging gym workouts, bodyweight, and exercise progress. This CLI is an unofficial tool for exporting and analyzing your own data.
