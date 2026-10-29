package csharp

import (
	"encoding/xml"
	"path/filepath"
	"strings"

	"github.com/archstats/archstats/core/module"
)

// ---------------------------------------------------------------------------
// dotnet: C#
//
// The `.csproj` is .NET's real module, and in nopCommerce it is the whole
// architecture: 40 projects named `Nop.Core`, `Nop.Services`, `Nop.Data`,
// `Nop.Web`, and `Nop.Plugin.Payments.PayPal` and thirty more like it. The
// namespaces do not say which of those is a plugin. The project references
// do, and they are what makes "core must never depend on a plugin"
// checkable.
// ---------------------------------------------------------------------------

type dotnetReader struct{}

func (dotnetReader) Kind() string { return "dotnet" }
func (dotnetReader) Claims(b string) bool {
	switch strings.ToLower(filepath.Ext(b)) {
	case ".csproj", ".fsproj", ".vbproj":
		return true
	}
	return false
}

type msbuildProject struct {
	ItemGroups []struct {
		ProjectReference []struct {
			Include string `xml:"Include,attr"`
		} `xml:"ProjectReference"`
		PackageReference []struct {
			Include string `xml:"Include,attr"`
		} `xml:"PackageReference"`
	} `xml:"ItemGroup"`
}

func (dotnetReader) Read(root, path string) []*module.Module {
	raw, ok := module.ReadFile(path)
	if !ok {
		return nil
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if name == "" {
		return nil
	}
	var proj msbuildProject
	var deps []string
	// A malformed or unusual project file still declares a module by its own
	// name; only its dependencies are lost.
	if xml.Unmarshal([]byte(raw), &proj) == nil {
		for _, group := range proj.ItemGroups {
			for _, ref := range group.ProjectReference {
				// `..\Nop.Core\Nop.Core.csproj` names the project, not a path
				// anybody downstream cares about.
				p := strings.ReplaceAll(ref.Include, `\`, "/")
				deps = module.AppendUnique(deps, strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)))
			}
			for _, ref := range group.PackageReference {
				deps = module.AppendUnique(deps, ref.Include)
			}
		}
	}
	return []*module.Module{{
		Name:      name,
		Dir:       module.RelDir(root, path),
		Manifest:  module.RelFile(root, path),
		DependsOn: deps,
	}}
}

// ManifestReaders reads .NET project files.
func ManifestReaders() []module.Reader { return []module.Reader{dotnetReader{}} }
