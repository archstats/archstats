// Package snapdigest reduces a snapshot to what the scan said, so two scans
// can be compared table by table: a golden digest pins the fixtures' scans in
// the tests, and scripts/scan-parity.sh compares a refactor's scans with the
// commit before it.
//
// What is left out is what says when or by what the scan ran, not what it
// found: the scan time and report id every table carries, and the engine
// version and analysis revision in _snapshot (the digest file records the
// revision in its header). Floats keep six
// significant digits, so the last bits of a centrality computed with fused
// multiply-add on arm64 read the same as on amd64.
package snapdigest

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// Table is one table or view of a snapshot: its columns and every row,
// rendered and sorted.
type Table struct {
	Name    string
	Columns []string
	Rows    []string
}

// The _snapshot keys that say how and when, not what.
var volatileKeys = map[string]bool{
	"scanned_at":        true,
	"report_id":         true,
	"engine_version":    true,
	"analysis_revision": true,
}

// Options leaves columns out of the comparison, as "table.column" or
// "*.column".
type Options struct {
	Ignore []string
}

// Every table carries the scan's report id and time as column defaults.
var volatileColumns = map[string]bool{"report_id": true, "timestamp": true}

func (o Options) ignored(table, column string) bool {
	if volatileColumns[column] {
		return true
	}
	for _, i := range o.Ignore {
		t, c, ok := strings.Cut(i, ".")
		if ok && (t == table || t == "*") && c == column {
			return true
		}
	}
	return false
}

// Read loads every table and view of the snapshot at dbFile.
func Read(dbFile string, opts Options) ([]Table, error) {
	db, err := sql.Open("sqlite3", "file:"+dbFile+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	names, err := tableNames(db)
	if err != nil {
		return nil, err
	}
	var tables []Table
	for _, name := range names {
		t, err := readTable(db, name, opts)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		tables = append(tables, t)
	}
	return tables, nil
}

func tableNames(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func readTable(db *sql.DB, name string, opts Options) (Table, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT * FROM `%s`", name))
	if err != nil {
		return Table{}, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return Table{}, err
	}
	t := Table{Name: name}
	var keep []int
	keyColumn := -1
	for i, c := range columns {
		if name == "_snapshot" && c == "key" {
			keyColumn = i
		}
		if !opts.ignored(name, c) {
			keep = append(keep, i)
			t.Columns = append(t.Columns, c)
		}
	}
	values := make([]interface{}, len(columns))
	pointers := make([]interface{}, len(columns))
	for i := range values {
		pointers[i] = &values[i]
	}
	for rows.Next() {
		if err := rows.Scan(pointers...); err != nil {
			return Table{}, err
		}
		if keyColumn >= 0 && volatileKeys[plain(values[keyColumn])] {
			continue
		}
		cells := make([]string, len(keep))
		for j, i := range keep {
			cells[j] = render(values[i])
		}
		t.Rows = append(t.Rows, strings.Join(cells, "\t"))
	}
	sort.Strings(t.Rows)
	return t, rows.Err()
}

// plain is a cell as text, unquoted.
func plain(v interface{}) string {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return fmt.Sprint(v)
}

func render(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case float64:
		return strconv.FormatFloat(x, 'g', 6, 64)
	case []byte:
		return strconv.Quote(string(x))
	case string:
		return strconv.Quote(x)
	default:
		return fmt.Sprint(x)
	}
}

// Digest is one line per table: its name, row count and a hash of its
// columns and rows. Two digests that agree on a line agree on the table.
func Digest(tables []Table) string {
	var b strings.Builder
	for _, t := range tables {
		h := sha256.New()
		h.Write([]byte(strings.Join(t.Columns, "\t")))
		for _, r := range t.Rows {
			h.Write([]byte{'\n'})
			h.Write([]byte(r))
		}
		fmt.Fprintf(&b, "%s rows=%d sha=%s\n", t.Name, len(t.Rows), hex.EncodeToString(h.Sum(nil))[:16])
	}
	return b.String()
}

// Diff says how two snapshots differ: tables only one has, columns that
// changed, and up to max rows each side has that the other lacks. An empty
// result means they agree.
func Diff(a, b []Table, max int) string {
	byName := func(ts []Table) map[string]Table {
		m := map[string]Table{}
		for _, t := range ts {
			m[t.Name] = t
		}
		return m
	}
	am, bm := byName(a), byName(b)
	names := map[string]bool{}
	for n := range am {
		names[n] = true
	}
	for n := range bm {
		names[n] = true
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)

	var out strings.Builder
	for _, n := range sorted {
		ta, inA := am[n]
		tb, inB := bm[n]
		switch {
		case !inA:
			fmt.Fprintf(&out, "== %s: only in the second (%d rows)\n", n, len(tb.Rows))
			continue
		case !inB:
			fmt.Fprintf(&out, "== %s: only in the first (%d rows)\n", n, len(ta.Rows))
			continue
		}
		if strings.Join(ta.Columns, ",") != strings.Join(tb.Columns, ",") {
			fmt.Fprintf(&out, "== %s: columns differ\n  - %s\n  + %s\n", n, strings.Join(ta.Columns, ","), strings.Join(tb.Columns, ","))
			continue
		}
		onlyA, onlyB := minus(ta.Rows, tb.Rows), minus(tb.Rows, ta.Rows)
		if len(onlyA) == 0 && len(onlyB) == 0 {
			continue
		}
		fmt.Fprintf(&out, "== %s: %d rows → %d rows, %d only in the first, %d only in the second\n  columns: %s\n",
			n, len(ta.Rows), len(tb.Rows), len(onlyA), len(onlyB), strings.Join(ta.Columns, "\t"))
		for i, r := range onlyA {
			if i == max {
				fmt.Fprintf(&out, "  - … %d more\n", len(onlyA)-max)
				break
			}
			fmt.Fprintf(&out, "  - %s\n", r)
		}
		for i, r := range onlyB {
			if i == max {
				fmt.Fprintf(&out, "  + … %d more\n", len(onlyB)-max)
				break
			}
			fmt.Fprintf(&out, "  + %s\n", r)
		}
	}
	return out.String()
}

// minus is the rows of a not in b, counting duplicates; both sorted.
func minus(a, b []string) []string {
	var out []string
	i, j := 0, 0
	for i < len(a) {
		switch {
		case j == len(b) || a[i] < b[j]:
			out = append(out, a[i])
			i++
		case a[i] > b[j]:
			j++
		default:
			i++
			j++
		}
	}
	return out
}
