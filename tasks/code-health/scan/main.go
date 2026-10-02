// Scans a folder exactly as archstats-ui/app/scan does (extensionsFor + Analyze +
// RenderView + SaveToDB), against the engine working tree. Scratch only.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/archstats/archstats/cmd/common"
	"github.com/archstats/archstats/cmd/config"
	"github.com/archstats/archstats/cmd/export/sqlite"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/walker"
	"github.com/spf13/cobra"
)

func main() {
	root, out := os.Args[1], os.Args[2]
	files, err := walker.GetAllFiles(root)
	must(err)
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.Path())
	}
	ctx := &config.DiscoveryContext{RootDir: root, Files: paths}
	configured := common.AlwaysEnabled()
	var names []string
	for _, opt := range common.Optional() {
		if opt.DiscoveryTrigger != nil && opt.DiscoveryTrigger(ctx) {
			configured = append(configured, opt)
			names = append(names, opt.Name)
		}
	}
	cmd := &cobra.Command{}
	fl := cmd.Flags()
	for _, ce := range configured {
		for param, arg := range ce.Arguments {
			if fl.Lookup(param) != nil {
				continue
			}
			switch arg.Type {
			case config.String:
				fl.String(param, arg.Default.(string), arg.Description)
			case config.Int:
				fl.Int(param, arg.Default.(int), arg.Description)
			case config.Bool:
				fl.Bool(param, arg.Default.(bool), arg.Description)
			case config.StringSlice:
				fl.StringSlice(param, arg.Default.([]string), arg.Description)
			}
		}
	}
	var exts []core.Extension
	for _, ce := range configured {
		ext, err := ce.Initializer(cmd)
		must(err)
		exts = append(exts, ext)
	}
	fmt.Println("extensions discovered:", names)
	res, err := core.New(&core.Config{RootPath: root, Extensions: exts}).Analyze()
	must(err)
	var views []*core.View
	for _, vf := range res.GetViewFactories() {
		v, err := res.RenderView(vf.Name)
		must(err)
		views = append(views, v)
	}
	os.Remove(out)
	must(sqlite.SaveToDB(&sqlite.SqlOptions{DatabaseName: out, ReportId: "audit", ScanTime: time.Now(), StoreContent: false}, res, views))
	fmt.Println("saved", out)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
