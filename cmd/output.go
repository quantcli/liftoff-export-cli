package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"

	"github.com/spf13/cobra"
)

// printJSON encodes v as indented JSON on w.
//
// A nil slice marshals as "null"; force "[]" so empty windows match the
// CONTRACT §4 empty-result rule (and don't blow up jq pipelines).
func printJSON(w io.Writer, v any) error {
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Slice && rv.IsNil() {
		_, err := io.WriteString(w, "[]\n")
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("failed to encode output: %w", err)
	}
	return nil
}

// emptyNotice tells the user their window matched nothing, on stderr.
//
// CONTRACT §4 says stdout is "data only" and that an empty result is
// "success with empty output (`[]` for JSON, no rows for markdown/CSV),
// exit code 0". A chatty "No workouts found." line on stdout breaks both
// halves of that: it is not a row, and `liftoff-export workouts list >
// out.md` ends up with prose where a consumer expects fitdown.
//
// Silence on a terminal is its own bug though (#24) — the user cannot
// tell an empty window from a crash. stderr serves both audiences: a
// human still sees the line, while `>` redirects and pipelines see
// nothing. (#23, #24, #37)
func emptyNotice(cmd *cobra.Command, format string, args ...any) {
	fmt.Fprintf(cmd.ErrOrStderr(), format+"\n", args...)
}

// countNoun renders "1 session" / "2 sessions" rather than always
// pluralizing. Only handles nouns that pluralize with a bare "s", which
// is all this CLI needs. (#28)
func countNoun(n int, singular string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %ss", n, singular)
}
