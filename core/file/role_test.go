package file

import "testing"

func TestRole(t *testing.T) {
	cases := map[string]string{
		"src/main/java/com/acme/Order.java":           RoleProduction,
		"src/test/java/com/acme/OrderTest.java":       RoleTest,
		"core/src/main/java/com/acme/OrderIT.java":    RoleTest,
		"src/Sylius/Behat/Context/ProductContext.php": RoleTest,
		"tests/Api/OrderTest.php":                     RoleTest,
		"src/Nop.Tests/Nop.Services.Tests/Foo.cs":     RoleTest,
		"src/Libraries/Nop.Services/Foo.cs":           RoleProduction,
		"src/oscar/apps/basket/tests/test_models.py":  RoleTest,
		"src/oscar/conftest.py":                       RoleTest,
		"./context_test.go":                           RoleTest,
		"testdata/protoexample/test.proto":            RoleTest,
		"client/src/components/Nav.spec.tsx":          RoleTest,
		"client/src/__tests__/nav.ts":                 RoleTest,
		"client/src/style.css":                        RoleNonCode,
		"src/oscar/locale/fr/LC_MESSAGES/django.po":   RoleNonCode,
		"features/checkout.feature":                   RoleTest,
		"client/src/Latest.tsx":                       RoleProduction,
		"src/contest/Winner.java":                     RoleProduction,
		"docs/attestation.md":                         RoleNonCode,
	}
	for p, want := range cases {
		if got := Role(p, false, false); got != want {
			t.Errorf("%s: %s, want %s", p, got, want)
		}
	}
	if got := Role("tests/vendor.min.js", true, false); got != RoleThirdParty {
		t.Errorf("third party wins, got %s", got)
	}
	if got := Role("migrations/0001_initial.py", false, true); got != RoleGenerated {
		t.Errorf("generated wins over the rest, got %s", got)
	}
}
