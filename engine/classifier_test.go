package engine

import (
	"strings"
	"testing"

	"github.com/archstats/archstats/core/file"
	"github.com/stretchr/testify/assert"
)

// The catalogue's conventions, together, judge files exactly as core did
// when it held them itself (ADR 0025): these cases are the ones core's own
// tests used to hold.
var files = Classifier()

func TestRole(t *testing.T) {
	cases := map[string]string{
		"src/main/java/com/acme/Order.java":           file.RoleProduction,
		"src/test/java/com/acme/OrderTest.java":       file.RoleTest,
		"core/src/main/java/com/acme/OrderIT.java":    file.RoleTest,
		"src/Sylius/Behat/Context/ProductContext.php": file.RoleTest,
		"tests/Api/OrderTest.php":                     file.RoleTest,
		"src/Nop.Tests/Nop.Services.Tests/Foo.cs":     file.RoleTest,
		"src/Libraries/Nop.Services/Foo.cs":           file.RoleProduction,
		"src/oscar/apps/basket/tests/test_models.py":  file.RoleTest,
		"src/oscar/conftest.py":                       file.RoleTest,
		"./context_test.go":                           file.RoleTest,
		"testdata/protoexample/test.proto":            file.RoleTest,
		"client/src/components/Nav.spec.tsx":          file.RoleTest,
		"client/src/__tests__/nav.ts":                 file.RoleTest,
		"client/src/style.css":                        file.RoleNonCode,
		"src/oscar/locale/fr/LC_MESSAGES/django.po":   file.RoleNonCode,
		"features/checkout.feature":                   file.RoleTest,
		"client/src/Latest.tsx":                       file.RoleProduction,
		"src/contest/Winner.java":                     file.RoleProduction,
		"docs/attestation.md":                         file.RoleNonCode,
		"app/src/androidTest/java/com/acme/UiTest.kt": file.RoleTest,
		"shared/src/commonTest/kotlin/Repo.kt":        file.RoleTest,
		"shared/src/commonMain/kotlin/Repo.kt":        file.RoleProduction,
		"Wondrous/WondrousTests/StoreTests.swift":     file.RoleTest,
		"Wondrous/WondrousUITests/Launch.swift":       file.RoleTest,
		"Sources/Model/Store.swift":                   file.RoleProduction,
		"App/Latest.swift":                            file.RoleProduction,
		"App/LoginViewControllerTests.m":              file.RoleTest,
		"lib/contests/Contests.swift":                 file.RoleProduction,
		"test/widget_test.dart":                       file.RoleTest,
		"lib/data/repo_test.dart":                     file.RoleTest,
	}
	for p, want := range cases {
		if got := files.Role(p, false, false, ""); got != want {
			t.Errorf("%s: %s, want %s", p, got, want)
		}
	}
	if got := files.Role("tests/vendor.min.js", true, false, ""); got != file.RoleThirdParty {
		t.Errorf("third party wins, got %s", got)
	}
	if got := files.Role("migrations/0001_initial.py", false, true, ""); got != file.RoleGenerated {
		t.Errorf("generated wins over the rest, got %s", got)
	}
}

func TestThirdPartyByPath(t *testing.T) {
	for _, p := range []string{
		"src/Presentation/Nop.Web/wwwroot/lib_npm/chart.js/chart.umd.min.js",
		"admin/src/main/resources/open_admin_style/js/admin/vendor/moment-with-locales.min.js",
		"admin/src/main/resources/open_admin_style/js/admin/lib/redactor.js",
		"common/src/main/resources/common_style/js/lib/jquery-3.5.1.js",
		"common/src/main/resources/common_style/js/lib/plugins/jquery-ui-1.13.3.custom.js",
		"web/static/js/bootstrap.bundle.js",
		"node_modules/react/index.js",
		"vendor/github.com/x/y/z.go",
		"app/assets/app.min.css",
	} {
		assert.True(t, files.IsThirdParty(p, nil), p)
	}
}

func TestOwnCodeIsNotThirdParty(t *testing.T) {
	for _, p := range []string{
		"client/src/components/Chat/ChatForm.tsx",
		"admin/src/main/resources/open_admin_style/js/admin/lib/plugins/blc-admin-filter-builder.js",
		"src/charts/chart-panel.js",
		"src/utils/moment-utils.ts",
		"api/server/index.js",
		"src/main/java/org/example/VendorService.java",
	} {
		assert.False(t, files.IsThirdParty(p, []byte("const a = 1;\nexport default a;\n")), p)
	}
}

func TestMinifiedContentIsThirdParty(t *testing.T) {
	minified := []byte("!function(e,t){" + strings.Repeat("var a=1;", 400) + "}();\n")
	assert.True(t, files.IsThirdParty("static/app-legacy.js", minified))
	readable := []byte(strings.Repeat("function f() {\n  return 1;\n}\n", 100))
	assert.False(t, files.IsThirdParty("static/app-legacy.js", readable))
}

func TestPackagesNamedLikeVendorDirsAreOwnCode(t *testing.T) {
	assert.False(t, files.IsThirdParty("common/src/main/java/org/broadleafcommerce/common/vendor/service/AbstractVendorService.java", nil))
	assert.True(t, files.IsThirdParty("vendor/github.com/pkg/errors/errors.go", nil))
	assert.True(t, files.InDirOutsideSourceRoot("module/target/generated-sources/x/y.java", "target/"))
	assert.False(t, files.InDirOutsideSourceRoot("module/src/main/java/org/x/build/buildservice.java", "build/"))
	assert.True(t, files.InDirOutsideSourceRoot("client/build/static/js/main.js", "build/"))
}

// A distributed library says so in its banner: a licence and a release.
func TestALibraryBannerIsThirdParty(t *testing.T) {
	libraries := map[string]string{
		"src/Plugins/Forums/Content/js/purify.js": "/*! @license DOMPurify 3.4.14 | (c) Cure53 and other contributors | Released under the Apache license 2.0 */\n(function (global, factory) {})\n",
		"admin/js/admin/lib/plugins/spectrum.js":  "// Spectrum Colorpicker v1.8.1\n// https://github.com/bgrins/spectrum\n// Author: Brian Grinstead\n// License: MIT\n\n(function (factory) {})\n",
		"css/swiper-bundle.css":                   "\xef\xbb\xbf/**\n * Swiper 14.2.0\n * Copyright 2014-2026 Vladimir Kharlampidi\n * Released under the MIT License\n */\n.swiper {}\n",
	}
	for path, src := range libraries {
		assert.Truef(t, files.IsThirdParty(path, []byte(src)), "%s", path)
	}
	own := map[string]string{
		// Broadleaf's own header: a licence, but no release number.
		"admin/js/admin/lib/plugins/blc-admin-filter-builder.js": "/*-\n * #%L\n * BroadleafCommerce Open Admin Platform\n * Copyright (C) 2009 - 2026 Broadleaf Commerce\n * Licensed under the Broadleaf Fair Use License Agreement, Version 1.0\n */\n(function($) {})\n",
		// A version in a comment that is not a licence banner.
		"client/src/utils/semver.ts.js": "// Parses versions like 1.2.3\nexport function parse(v) {}\n",
		// A banner further down the file is not this file's banner.
		"client/src/app.js": "import x from './x'\n/* (c) Vendor 1.2.3 */\n",
	}
	for path, src := range own {
		assert.Falsef(t, files.IsThirdParty(path, []byte(src)), "%s", path)
	}
}

func TestGeneratedFilesAreRecognisedByTheirHeader(t *testing.T) {
	generated := []string{
		"\xef\xbb\xbf//----------------------\n// <auto-generated>\n//     Generated using the NSwag toolchain\n// </auto-generated>\n",
		"// Code generated by protoc-gen-go. DO NOT EDIT.\n\npackage x\n",
		"# Generated by Django 3.2.4 on 2021-06-11 09:00\n\nfrom django.db import migrations\n",
	}
	for _, src := range generated {
		assert.Truef(t, files.IsGenerated([]byte(src)), "%q", src)
	}
	handWritten := []string{
		// Broadleaf's JPA entities.
		"@Entity\npublic class EmailTrackingImpl {\n  @Id\n  @GeneratedValue(generator = \"EmailTrackingId\")\n",
		"// This comment says Code generated by hand, and nothing else.\n",
		"",
	}
	for _, src := range handWritten {
		assert.Falsef(t, files.IsGenerated([]byte(src)), "%q", src)
	}
}
