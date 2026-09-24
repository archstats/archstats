package java

import (
	"testing"

	"github.com/archstats/archstats/core/unit"
	"github.com/stretchr/testify/assert"
)

const refsSample = `package org.acme.orders;

import org.acme.money.Money;
import org.acme.audit.*;
import static org.acme.util.Strings.join;

@Tracked
public class OrderServiceImpl implements OrderService {
    private OrderRepository repo;
    public Money total() { return Money.zero(); }
    public String label() { return OrderStatus.OPEN.name() + join(",", "a"); }
    public Auditor auditor() { return null; }
}
`

// Java names its same-package types without an import, and on-demand imports
// name no type at all; both are references, resolved exactly where the
// compiler looks. Only imports used to count, so every one of Broadleaf's
// 928 implements/extends of a same-package type was missing.
func TestReferencesResolveLikeTheCompiler(t *testing.T) {
	res := createJavaLanguagePack().AnalyzeFileContent("OrderServiceImpl.java", []byte(refsSample))
	refs := javaRefs(res.Snippets, "org.acme.orders")

	assert.Contains(t, refs, unit.Ref{Module: "org.acme.money", Name: "Money"})
	assert.Contains(t, refs, unit.Ref{Module: "org.acme.util", Name: "Strings"}, "a static import is a reference to its class")
	for _, name := range []string{"OrderService", "OrderRepository", "OrderStatus", "Tracked", "Auditor"} {
		assert.Contains(t, refs, unit.Ref{Module: "org.acme.orders", Name: name, Exact: true}, name)
	}
	assert.Contains(t, refs, unit.Ref{Module: "org.acme.audit", Name: "Auditor", Exact: true}, "on-demand import")
	// An explicitly imported name is not also looked up in the own package.
	assert.NotContains(t, refs, unit.Ref{Module: "org.acme.orders", Name: "Money", Exact: true})
	// Nor is the class itself, nor a lower-case member.
	assert.NotContains(t, refs, unit.Ref{Module: "org.acme.orders", Name: "OrderServiceImpl", Exact: true})
	assert.NotContains(t, refs, unit.Ref{Module: "org.acme.orders", Name: "repo", Exact: true})
	// The helper captures do not leave the analyser.
	for _, s := range withoutReferenceCaptures(res.Snippets) {
		assert.NotContains(t, s.Type, "java__ref__")
	}
}

// An annotation type is a type: 63 of Broadleaf's were missing as units, and
// with them 762 imports.
func TestAnnotationTypesAreDeclarations(t *testing.T) {
	res := createJavaLanguagePack().AnalyzeFileContent("Tracked.java", []byte("package org.acme;\npublic @interface Tracked { String value() default \"\"; }\n"))
	found := false
	for _, s := range res.Snippets {
		if s.Type == "java__type__declaration" && s.Value == "Tracked" {
			found = true
		}
	}
	assert.True(t, found)
}
