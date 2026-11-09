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
)

// liftoffFormatSubcommands is the §4 surface for liftoff — each
// data-producing leaf owns its own --format flag under a two-level
// cobra path (parent group + leaf). The compat Runner splits on
// whitespace so cobra sees separate argv entries (added in
// quantcli/common PR #12).
//
// `workouts show` is intentionally excluded: it requires a positional
// <date> argument, so the parse-level subtests would fail on missing
// args rather than the --format flag itself. The four leaves below
// take --format with no required positional and exercise the §4
// surface cleanly.
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
	// liftoff implements --format markdown (default) and --format json
	// today; CSV is not yet wired. SupportedFormats: ["markdown","json"]
	// skips CSVHasHeader with a named reason rather than failing it.
	//
	// SkipDataPath: true opts out of JSONIsArray / CSVHasHeader /
	// DefaultIsMarkdown — liftoff's data path requires a stored OAuth
	// token at ~/.config/liftoff-export/auth.json which the compat CI
	// job does not provision, so a clean `--format json` run exits
	// non-zero with "not logged in" before the JSON-array assertion
	// could run. The parse-level subtests (HelpDocumentsFormatFlag,
	// UnknownFormatFails, FlagValidationIsHermetic) still attest the
	// §4 surface.
	formats.RunContract(t, compat.Runner{
		Binary:           bin,
		Subcommands:      liftoffFormatSubcommands,
		SupportedFormats: []string{"markdown", "json"},
		SkipDataPath:     true,
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
