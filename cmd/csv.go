package cmd

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"strconv"
	"time"
)

// The `--format csv` codec. (#26)
//
// CONTRACT §4 says only that csv is "spreadsheet-friendly", that stdout is
// data only, and (§2) that CSV timestamps are rendered in the user's local
// zone. compat/formats adds one machine-checked rule: a header row is
// emitted even when the result is empty. Everything else below follows
// withings-export-cli, the one exporter in the family that already ships
// csv, so the CLIs agree where the contract is silent:
//
//   - RFC 4180 via encoding/csv with its defaults: comma delimiter, LF
//     record terminator, a field quoted only when it contains a comma, a
//     quote, a newline or leading space, embedded quotes doubled. UTF-8,
//     no BOM.
//   - Exactly one header row, always — including for an empty result, so a
//     consumer never has to special-case "no file" vs "no rows".
//   - snake_case column names. Each column is the snake_case form of the
//     JSON key it carries, so the two codecs describe the same fields.
//   - One row per data point. Nested JSON is denormalised to the deepest
//     repeated element (a set, a session, a month) with the parent's
//     scalar columns repeated on every row, the way a SQL join would.
//   - Numbers are bare decimals with no thousands separator and no unit
//     suffix (strconv 'f', shortest round-trip), so a spreadsheet types
//     the column as numeric.
//   - Timestamps are RFC3339 in the local zone; calendar days are
//     YYYY-MM-DD.
//   - An absent value (JSON null, or a key JSON omits) is an empty cell —
//     never 0, for the same reason §4 "Scalar typing" forbids 0 in JSON.

// writeCSV writes header and then rows to w as RFC 4180 CSV.
//
// The header is written unconditionally: a nil or empty rows is the §4
// empty result, which for csv is the header alone.
func writeCSV(w io.Writer, header []string, rows [][]string) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(header); err != nil {
		return err
	}
	return cw.WriteAll(rows) // flushes, and reports any write error
}

// csvFloat renders a number the way strconv round-trips it: no exponent,
// no trailing zeros, no float noise beyond what the value actually holds.
func csvFloat(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// csvFloatPtr is csvFloat for a nullable number; nil is an empty cell.
func csvFloatPtr(v *float64) string {
	if v == nil {
		return ""
	}
	return csvFloat(*v)
}

// csvIntPtr renders a nullable integer; nil is an empty cell.
func csvIntPtr(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

// csvNumberPtr renders a nullable json.Number; nil is an empty cell. The
// digits pass through verbatim, the way input_one / input_two already do
// — csvFloat would re-render them through a float64 round trip, and a
// unit sibling that disagrees in its last decimal place with the bare
// field it was projected from is worse than no sibling at all.
func csvNumberPtr(n *json.Number) string {
	if n == nil {
		return ""
	}
	return n.String()
}

// csvStrPtr renders a nullable string; nil is an empty cell. CSV cannot
// tell a null string from an empty one — --format json keeps that
// distinction for callers that need it.
func csvStrPtr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// csvOmitZeroFloat and csvOmitZeroInt mirror `json:",omitempty"`: where
// the JSON codec drops a zero-valued key because the stat does not apply
// to that row, the csv cell is empty rather than a misleading 0.
func csvOmitZeroFloat(v float64) string {
	if v == 0 {
		return ""
	}
	return csvFloat(v)
}

func csvOmitZeroInt(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}

// csvLocalTime renders an upstream RFC3339 timestamp in the local zone
// (CONTRACT §2). A value that does not parse is passed through unchanged
// so the row still shows what the API sent.
func csvLocalTime(s string) string {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return s
	}
	return t.Local().Format(time.RFC3339)
}

// csvLocalDate renders the local calendar day of an upstream timestamp,
// matching the day the markdown renderer and the --since/--until window
// bucket it under. Unparseable input yields an empty cell.
func csvLocalDate(s string) string {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return ""
	}
	return t.Local().Format("2006-01-02")
}

// csvPad returns row extended with empty cells to n columns. Used for a
// parent that has no children to join against (a workout with no
// exercises), so every record still has the header's field count.
func csvPad(row []string, n int) []string {
	out := make([]string, n)
	copy(out, row)
	return out
}
