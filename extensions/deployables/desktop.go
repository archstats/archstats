package deployables

import (
	"encoding/json"
	stdpath "path"
	"sort"
)

// Desktop apps are deployables the way mobile apps are: what ships is an
// installer or a binary built from the whole project, frontend included.
// Three frameworks say so in a file of their own: Wails (wails.json beside
// the Go module), Tauri (src-tauri/tauri.conf.json under the web project)
// and Electron (a Node module that depends on electron).

type desktopApp struct {
	names    []string
	dir      string
	file     string
	platform string
}

// readWails reads a wails.json: the app is the folder it sits in.
func readWails(file string, content []byte) *desktopApp {
	var cfg struct {
		Name           string `json:"name"`
		OutputFilename string `json:"outputfilename"`
		Info           struct {
			ProductName string `json:"productName"`
		} `json:"info"`
	}
	if json.Unmarshal(content, &cfg) != nil {
		return nil
	}
	return &desktopApp{names: nonEmpty(cfg.Name, cfg.OutputFilename, cfg.Info.ProductName), dir: dirOf(file), file: file, platform: "wails"}
}

// readTauri reads a tauri.conf.json: the app is the web project around its
// src-tauri folder. Tauri 1 keeps the name under package, Tauri 2 at the top.
func readTauri(file string, content []byte) *desktopApp {
	var cfg struct {
		ProductName string `json:"productName"`
		Identifier  string `json:"identifier"`
		Package     struct {
			ProductName string `json:"productName"`
		} `json:"package"`
	}
	if json.Unmarshal(content, &cfg) != nil {
		return nil
	}
	dir := dirOf(file)
	if stdpath.Base(dir) == "src-tauri" {
		dir = dirOf(dir)
	}
	return &desktopApp{names: nonEmpty(cfg.ProductName, cfg.Package.ProductName, cfg.Identifier), dir: dir, file: file, platform: "tauri"}
}

func nonEmpty(xs ...string) []string {
	var out []string
	for _, x := range xs {
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}

func (b *builder) desktopDeployables() {
	apps := append([]*desktopApp(nil), b.desktop...)
	if b.in.Modules != nil {
		for _, mod := range b.in.Modules.Modules() {
			if mod.Kind == "node" && hasDep(mod, "electron") {
				apps = append(apps, &desktopApp{names: []string{mod.Name}, dir: cleanDir(mod.Dir), file: mod.Manifest, platform: "electron"})
			}
		}
	}
	sort.SliceStable(apps, func(i, j int) bool { return apps[i].file < apps[j].file })
	claimed := map[string]bool{}
	for _, a := range apps {
		if claimed[a.dir] || b.skipped(a.file) || b.contained(a.dir) {
			continue
		}
		claimed[a.dir] = true
		d := &Deployable{Kind: "desktop_app", Platform: a.platform, BuiltBy: a.platform, File: a.file, Line: 1, Context: a.dir, priority: 5}
		d.aliases = a.names
		if len(d.aliases) == 0 {
			d.aliases = []string{stdpath.Base(a.dir)}
		}
		// The module the app is: its Go module, or its package.json.
		if b.in.Modules != nil {
			for _, mod := range b.in.Modules.Modules() {
				if cleanDir(mod.Dir) == a.dir {
					d.modules = append(d.modules, mod.Name)
				}
			}
		}
		b.m.Deployables = append(b.m.Deployables, d)
		b.addContent(d, &Content{Path: a.dir, File: a.file, Line: 1, Resolution: "declared"})
	}
}
