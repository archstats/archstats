// Scans a folder the way a front end does (engine.Analyze +
// RenderView + SaveToDB), against the engine working tree. Scratch only.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/archstats/archstats/cmd/export/sqlite"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/engine"
)

func main() {
	root, out := os.Args[1], os.Args[2]
	_, names, err := engine.Select(root, nil, nil)
	must(err)
	fmt.Println("extensions discovered:", names)
	res, err := engine.Analyze(engine.Options{Root: root})
	must(err)
	var views []*core.View
	for _, vf := range res.GetViewFactories() {
		v, err := res.RenderView(vf.Name)
		must(err)
		views = append(views, v)
	}
	os.Remove(out)
	must(sqlite.SaveToDB(&sqlite.SqlOptions{DatabaseName: out, ReportId: "audit", ScanTime: time.Now(), StoreContent: true}, res, views))
	fmt.Println("saved", out)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
