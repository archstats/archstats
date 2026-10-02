package php

import (
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tsphp "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

func TestPHPCognitiveComplexity(t *testing.T) {
	stats, fns := complexity.MeasureByName(tree_sitter.NewLanguage(tsphp.LanguagePHP()), `<?php
use App\Models\User;
use App\Models\{Order, Line};

class A {
    public function f($a, $b) {
        if ($a) {                                             // +1
            foreach ($b as $x) { if ($x && $a) { return 1; } } // +2, +3, +1
        } elseif ($b) {                                       // +1
        } else if ($a > 1) {                                  // +1
        } else {                                              // +1
        }
        return $a ? 1 : 2;                                    // +1
    }
}
`)
	if f := fns["A.f"]; f == nil || f.Cognitive != 11 || f.Params != 2 {
		t.Errorf("A.f: %+v", f)
	}
	if stats["modularity__imports__count"] != 3 {
		t.Errorf("stats %v", stats)
	}
}
