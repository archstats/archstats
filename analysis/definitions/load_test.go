package definitions

import (
	"embed"
	"github.com/stretchr/testify/assert"
	"testing"
)

//go:embed definitions/**
var defs embed.FS

func TestLoadYamlFiles(t *testing.T) {

	definitions, err := LoadYamlFiles(defs)
	if err != nil {
		return
	}

	assert.Len(t, definitions, 3)

}
