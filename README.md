# liftoff-cli

A command-line interface for the [Liftoff](https://getgymbros.com) fitness app.

## Install

Download the latest release for your platform from the [releases page](https://github.com/DTTerastar/liftoff-cli/releases/latest), unzip it, and place the binary in `~/bin`.

**macOS (Apple Silicon):**
```sh
curl -L https://github.com/DTTerastar/liftoff-cli/releases/latest/download/liftoff_darwin_arm64.zip | unzip -d ~/bin -
chmod +x ~/bin/liftoff
```

**macOS (Intel):**
```sh
curl -L https://github.com/DTTerastar/liftoff-cli/releases/latest/download/liftoff_darwin_amd64.zip | unzip -d ~/bin -
chmod +x ~/bin/liftoff
```

**Linux (amd64):**
```sh
curl -L https://github.com/DTTerastar/liftoff-cli/releases/latest/download/liftoff_linux_amd64.zip | unzip -d ~/bin -
chmod +x ~/bin/liftoff
```

Make sure `~/bin` is in your `PATH`. If not, add this to your `~/.zshrc` or `~/.bashrc`:
```sh
export PATH="$HOME/bin:$PATH"
```

## Usage

**Log in:**
```sh
liftoff auth login
```

**List workouts (fitdown format):**
```sh
liftoff workouts list
```

**Filter by date:**
```sh
liftoff workouts list --since 30d
liftoff workouts list --since 2025-01-01
```

**Output as JSON:**
```sh
liftoff workouts list --json
```

**Show a single workout:**
```sh
liftoff workouts show <id>
```

**Log out:**
```sh
liftoff auth logout
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
