//go:build compat

// Compat-test entry point for liftoff-export-cli.
//
// This file is only compiled under the `compat` build tag, so it does
// not affect the default `go test ./...` run. CI invokes it as
// `go test -tags=compat ./...` after building the export binary and
// exposing its path through LIFTOFF_EXPORT_BIN.
//
// The actual assertions live in github.com/quantcli/common/compat.
// Drift between this CLI and CONTRACT.md surfaces as a failure here.
package main_test

import (
	"os"
	"testing"

	"github.com/quantcli/common/compat"
	"github.com/quantcli/common/compat/dates"
	"github.com/quantcli/common/compat/formats"
	"github.com/quantcli/liftoff-export-cli/internal/stubapi"
)

// liftoffFormatSubcommands is the §4 surface for liftoff — each
// data-producing leaf owns its own --format flag under a two-level
// cobra path (parent group + leaf). The compat Runner splits on
// whitespace so cobra sees separate argv entries (added in
// quantcli/common PR #12).
//
// `workouts show` is intentionally excluded: it requires a positional
// <date> argument, so the parse-level subtests would fail on missing
// args rather than the --format flag itself. Its §4 empty-result
// behaviour is covered in-process instead, by TestWorkoutsShowEmpty*
// in package cmd. The four leaves below take --format with no required
// positional and exercise the §4 surface cleanly.
var liftoffFormatSubcommands = []string{
	"workouts list",
	"workouts stats",
	"bodyweights list",
	"bodyweights stats",
}

func TestContractFormats(t *testing.T) {
	bin := os.Getenv("LIFTOFF_EXPORT_BIN")
	if bin == "" {
		t.Skip("LIFTOFF_EXPORT_BIN not set; skipping compat suite")
	}

	// The data-path subtests (JSONIsArray, DefaultIsMarkdown) need the
	// binary to reach a successful export, which previously meant a
	// Liftoff OAuth token this job has no way to provision — so they
	// were disabled via Runner.SkipDataPath and §4 was attested at
	// parse level only.
	//
	// stubapi closes that gap without credentials or network: it serves
	// the two tRPC procedures the data path calls, and the CLI's own
	// documented headless knobs (LIFTOFF_API_BASE + LIFTOFF_REFRESH_TOKEN)
	// point the child binary at it. Nothing in liftoff's code is mocked
	// — the binary under test is the one CI ships, running its real
	// fetch/filter/render path against a controlled payload.
	srv := stubapi.New(stubapi.SamplePosts)
	defer srv.Close()

	// Runner.Env replaces the child's environment wholesale, so this is
	// the complete set: the stub endpoint, a refresh token to take the
	// headless auth branch, a throwaway HOME so no developer's real
	// token file is read, and a fixed TZ so the fixture's local-day
	// bucketing renders identically on every runner.
	env := []string{
		"LIFTOFF_API_BASE=" + srv.BaseURL(),
		"LIFTOFF_REFRESH_TOKEN=compat-stub-refresh-token",
		"HOME=" + t.TempDir(),
		"TZ=UTC",
	}

	// liftoff implements --format markdown (default) and --format json
	// today; CSV is not yet wired. SupportedFormats: ["markdown","json"]
	// skips CSVHasHeader with a named reason rather than failing it.
	formats.RunContract(t, compat.Runner{
		Binary:           bin,
		Subcommands:      liftoffFormatSubcommands,
		SupportedFormats: []string{"markdown", "json"},
		Env:              env,
	})
}

// TestResolvedWindow verifies local dates independently of upstream credentials.
func TestResolvedWindow(t *testing.T) {
	bin := os.Getenv("LIFTOFF_EXPORT_BIN")
	if bin == "" {
		t.Skip("LIFTOFF_EXPORT_BIN not set")
	}
	dates.RunResolvedWindow(t, compat.Runner{Binary: bin, Subcommands: []string{"workouts list", "workouts stats", "bodyweights list", "bodyweights stats"}})
}
