package php

import (
	"encoding/json"

	"github.com/archstats/archstats/core/module"
)

// ---------------------------------------------------------------------------
// composer: PHP
//
// Sylius is 60 composer packages under one repository, and the split its
// whole design rests on -- `Sylius\Component\*` knowing nothing about
// `Sylius\Bundle\*` -- is a fact about those packages, not about namespaces.
// ---------------------------------------------------------------------------

type composerReader struct{}

func (composerReader) Kind() string         { return "composer" }
func (composerReader) Claims(b string) bool { return b == "composer.json" }

type composerManifest struct {
	Name     string            `json:"name"`
	Require  map[string]string `json:"require"`
	Autoload struct {
		PSR4 map[string]interface{} `json:"psr-4"`
	} `json:"autoload"`
}

func (composerReader) Read(root, path string) []*module.Module {
	raw, ok := module.ReadFile(path)
	if !ok {
		return nil
	}
	var man composerManifest
	if json.Unmarshal([]byte(raw), &man) != nil || man.Name == "" {
		return nil
	}
	return []*module.Module{{
		Name:      man.Name,
		Dir:       module.RelDir(root, path),
		Manifest:  module.RelFile(root, path),
		DependsOn: module.Keys(man.Require),
	}}
}

// ManifestReaders reads composer.json.
func ManifestReaders() []module.Reader { return []module.Reader{composerReader{}} }
