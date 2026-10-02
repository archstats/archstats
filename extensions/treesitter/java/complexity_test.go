package java

import (
	"testing"

	"github.com/archstats/archstats/core/file"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	java "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

func measureJava(src string) (map[string]interface{}, map[string]*file.Function) {
	return complexity.MeasureByName(tree_sitter.NewLanguage(java.Language()), src)
}

// The cases are hand-scored by SonarSource's rules; sumOfPrimes and getWords
// are the white paper's own examples, sumOfPrimes without its labelled
// continue, which this count leaves out.
func TestJavaCognitiveComplexity(t *testing.T) {
	stats, fns := measureJava(`package p;
import java.util.List;
import static java.util.Map.entry;

/** A class. */
class A {
  // a line comment
  int sum(int a, int b) {
    if (a > 0) {                                  // +1
      for (int i = 0; i < b; i++) {               // +2
        if (i % 2 == 0 && a > 1 && b > 2) { a++; } // +3, +1 for the run of &&
      }
    } else if (a < 0) {                           // +1
      a--;
    } else {                                      // +1
      a = 0;
    }
    return a;
  }

  int sumOfPrimes(int max) {
    int total = 0;
    for (int i = 1; i <= max; ++i) {    // +1
      for (int j = 2; j < i; ++j) {     // +2
        if (i % j == 0) {               // +3
          continue;
        }
      }
      total += i;
    }
    return total;
  }

  String getWords(int number) {
    switch (number) {                   // +1
      case 1: return "one";
      case 2: return "a couple";
      default: return "lots";
    }
  }

  Runnable later() {
    return () -> { if (ready) { run(); } }; // +2: the lambda nests the if
  }
}
`)
	for name, want := range map[string]int{"A.sum": 9, "A.sumOfPrimes": 6, "A.getWords": 1, "A.later": 2} {
		if f := fns[name]; f == nil || f.Cognitive != want {
			t.Errorf("%s: cognitive %v, want %d", name, f, want)
		}
	}
	if f := fns["A.sum"]; f.Params != 2 || f.Nesting != 3 || f.Begin != 8 || f.End != 19 {
		t.Errorf("A.sum: %+v", f)
	}
	if len(fns) != 4 {
		t.Errorf("functions %d, want 4 (the lambda is part of later)", len(fns))
	}
	// 45 lines: 4 blank, 2 comment-only; a trailing comment leaves its line code.
	if stats["complexity__lines__code"] != 39 || stats["modularity__imports__count"] != 2 {
		t.Errorf("code lines %v", stats["complexity__lines__code"])
	}
	if stats["complexity__functions__complex"] != 0 || stats["complexity__cognitive__max"] != 9 {
		t.Errorf("stats %v", stats)
	}
}

func TestJavaComplexLines(t *testing.T) {
	src := "class B {\n  void deep(int a) {\n"
	for i := 0; i < 5; i++ {
		src += "    if (a > 0) {\n      if (a > 1) {\n        if (a > 2) { a++; }\n      }\n    }\n" // 1+2+3 each
	}
	src += "  }\n}\n"
	stats, fns := measureJava(src)
	if fns["B.deep"].Cognitive != 30 {
		t.Fatalf("cognitive %d, want 30", fns["B.deep"].Cognitive)
	}
	if stats["complexity__functions__complex"] != 1 || stats["complexity__lines__complex"] != fns["B.deep"].Lines() {
		t.Errorf("stats %v, function lines %d", stats, fns["B.deep"].Lines())
	}
}
