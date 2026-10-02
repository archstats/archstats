package objc

import (
	"testing"

	objc "github.com/tree-sitter-grammars/tree-sitter-objc/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestObjCCognitiveComplexity(t *testing.T) {
	stats, fns := complexity.MeasureByName(tree_sitter.NewLanguage(objc.Language()), `#import <Foundation/Foundation.h>
#include "x.h"

@implementation Cart
- (int)total:(int)a name:(NSString *)b {
    if (a > 0 && b != nil) { for (int i = 0; i < a; i++) { } } else if (a < 0) { } else { } // 1+1+2+1+1
    while (a > 0) { }                            // +1
    switch (a) { case 1: break; default: break; } // +1
    @try { } @catch (NSException *e) { }         // +1
    [items enumerateObjectsUsingBlock:^(id x, NSUInteger i, BOOL *stop) { if (x) { } }]; // +2: the block nests
    return a > 1 ? 1 : 2;                        // +1
}
@end

int top(void) { return 1; }
`)
	if f := fns["Cart.total:name:"]; f == nil || f.Cognitive != 12 || f.Params != 2 {
		t.Errorf("Cart.total:name: %+v (all: %v)", f, fns)
	}
	if f := fns["top"]; f == nil || f.Params != 0 {
		t.Errorf("top: %+v", f)
	}
	if stats["modularity__imports__count"] != 2 || stats["complexity__functions"] != 2 {
		t.Errorf("stats %v", stats)
	}
}
