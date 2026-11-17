// Command snapdiff compares two snapshots table by table and exits 1 when
// they differ. scripts/scan-parity.sh runs it on the snapshots an old and a
// new binary take of the same checkout.
//
//	go run ./internal/snapdigest/snapdiff [-ignore table.column,...] [-rows 20] old.db new.db
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/archstats/archstats/internal/snapdigest"
)

func main() {
	ignore := flag.String("ignore", "", "columns to leave out, comma separated: table.column or *.column")
	rows := flag.Int("rows", 20, "rows to show per side of a table that differs")
	flag.Parse()
	if flag.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: snapdiff [-ignore table.column,...] [-rows N] old.db new.db")
		os.Exit(2)
	}
	var opts snapdigest.Options
	if *ignore != "" {
		opts.Ignore = strings.Split(*ignore, ",")
	}
	a, err := snapdigest.Read(flag.Arg(0), opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	b, err := snapdigest.Read(flag.Arg(1), opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if d := snapdigest.Diff(a, b, *rows); d != "" {
		fmt.Print(d)
		os.Exit(1)
	}
}
