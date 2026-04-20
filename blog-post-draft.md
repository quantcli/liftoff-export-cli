---
title: I pointed Claude at mitmproxy and it reverse-engineered my gym app's API
published: false
tags: ai, go, cli, fitness
---

I track my workouts in Liftoff, a gym tracking app. The app is great for logging sets and reps, but I wanted Claude to help me analyze my training — suggest programming changes, spot plateaus, flag imbalances. The problem: there's no export button. My data was locked inside the app.

So I asked Claude to figure it out.

## Claude runs mitmproxy

I told Claude I wanted to intercept my Liftoff app's traffic. It walked me through every step — installing mitmproxy, configuring my iPhone's proxy settings, installing the CA certificate for HTTPS decryption. All I did was follow instructions and tap through screens on my phone.

Then Claude ran mitmdump, told me to open Liftoff and scroll through my workouts, and read the captured traffic itself.

## Claude reverse-engineers the API

From the mitmdump output, Claude figured out that Liftoff uses tRPC — a TypeScript RPC framework that batches requests into a specific envelope format. The API lives at a versioned subdomain (`v2-12-2.api.getgymbros.com`) and expects an iOS user-agent string.

Claude mapped out the entire API:

- **Authentication flow** — a `user.signIn` endpoint that returns access and refresh tokens, with a `user.refreshToken` endpoint for renewal
- **The workout endpoint** — `post.getMyPosts` returns every workout you've ever logged
- **Exercise type codes** — `WR` for weight/reps, `AB` for assisted bodyweight, `BR` for bodyweight plus resistance, `DD` for distance/duration
- **Set data structure** — each set has a type (warmup vs working), and two inputs whose meaning changes based on exercise type

I didn't read a single packet. Claude did all of it.

## Claude builds the CLI

Once Claude understood the API, it wrote a Go CLI using Cobra. It handled auth token storage, automatic token refresh, and the tRPC request format.

The tool grew from there:

- `workouts list` — export workouts in fitdown or JSON, filter by date or exercise
- `workouts stats` — per-exercise volume summaries with monthly ASCII bar charts
- `bodyweights stats` — trend analysis with plateau detection

I set up goreleaser for multi-platform builds and a Homebrew tap for easy installation:

```sh
brew tap quantcli/tap
brew install liftoff-export
```

## The full circle

Here's the part I like most: the whole reason I built this was to pipe my workout data back to Claude.

Now I can run `liftoff-export workouts list --since 6m --json` and hand Claude six months of structured training data. It can see my actual sets, reps, and weights across every exercise. From there it helps me with programming — what to change, where I'm stalling, what's working.

Claude reverse-engineered the API, built the tool to extract my data, and now uses that data to coach my training. The AI closed its own loop.

## Links

- GitHub: [quantcli/liftoff-export-cli](https://github.com/quantcli/liftoff-export-cli)
- Liftoff app: [getgymbros.com](https://getgymbros.com)
