package file

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		assert.True(t, IsThirdParty(p, nil), p)
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
		assert.False(t, IsThirdParty(p, []byte("const a = 1;\nexport default a;\n")), p)
	}
}

func TestMinifiedContentIsThirdParty(t *testing.T) {
	minified := []byte("!function(e,t){" + strings.Repeat("var a=1;", 400) + "}();\n")
	assert.True(t, IsThirdParty("static/app-legacy.js", minified))
	readable := []byte(strings.Repeat("function f() {\n  return 1;\n}\n", 100))
	assert.False(t, IsThirdParty("static/app-legacy.js", readable))
}

func TestPackagesNamedLikeVendorDirsAreOwnCode(t *testing.T) {
	assert.False(t, IsThirdParty("common/src/main/java/org/broadleafcommerce/common/vendor/service/AbstractVendorService.java", nil))
	assert.True(t, IsThirdParty("vendor/github.com/pkg/errors/errors.go", nil))
	assert.True(t, InDirOutsideSourceRoot("module/target/generated-sources/x/y.java", "target/"))
	assert.False(t, InDirOutsideSourceRoot("module/src/main/java/org/x/build/buildservice.java", "build/"))
	assert.True(t, InDirOutsideSourceRoot("client/build/static/js/main.js", "build/"))
}

// A distributed library says so in its banner: a licence and a release.
func TestALibraryBannerIsThirdParty(t *testing.T) {
	libraries := map[string]string{
		"src/Plugins/Forums/Content/js/purify.js": "/*! @license DOMPurify 3.4.14 | (c) Cure53 and other contributors | Released under the Apache license 2.0 */\n(function (global, factory) {})\n",
		"admin/js/admin/lib/plugins/spectrum.js":  "// Spectrum Colorpicker v1.8.1\n// https://github.com/bgrins/spectrum\n// Author: Brian Grinstead\n// License: MIT\n\n(function (factory) {})\n",
		"css/swiper-bundle.css":                   "\xef\xbb\xbf/**\n * Swiper 14.2.0\n * Copyright 2014-2026 Vladimir Kharlampidi\n * Released under the MIT License\n */\n.swiper {}\n",
	}
	for path, src := range libraries {
		assert.Truef(t, IsThirdParty(path, []byte(src)), "%s", path)
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
		assert.Falsef(t, IsThirdParty(path, []byte(src)), "%s", path)
	}
}
