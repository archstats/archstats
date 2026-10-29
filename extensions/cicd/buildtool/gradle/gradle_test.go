package gradle

import (
	"testing"
)

func TestGradleProjects(t *testing.T) {
	cases := []struct {
		name, src, image, builtBy, java string
		exec                            bool
	}{
		{"kotlin dsl boot", `plugins { id("org.springframework.boot") version "3.2.0"; java }
java { toolchain { languageVersion.set(JavaLanguageVersion.of(21)) } }`, "", "", "21", true},
		{"groovy boot apply", "apply plugin: 'org.springframework.boot'\nsourceCompatibility = '17'\n", "", "", "17", true},
		{"jib block", "plugins { id 'com.google.cloud.tools.jib' version '3.4' }\njib {\n  to {\n    image = 'gcr.io/p/orders'\n  }\n}\n", "orders", "jib", "", false},
		{"jib default name", "plugins { id 'com.google.cloud.tools.jib' }\n", "svc", "jib", "", false},
		{"boot build image", "plugins { id 'org.springframework.boot' }\ntasks.named('bootBuildImage') {\n  imageName = \"acme/billing:${version}\"\n}\n", "billing", "spring-boot", "", true},
		{"library", "plugins { id 'java-library' }\nsourceCompatibility = JavaVersion.VERSION_1_8\n", "", "", "8", false},
		{"version catalog alias", "plugins { alias(libs.plugins.spring.boot) }\njvmToolchain(17)\n", "", "", "17", true},
	}
	for _, c := range cases {
		g := readGradle("svc/build.gradle", []byte(c.src), "svc")
		image, by := "", ""
		if len(g.Images) > 0 {
			image, by = g.Images[0].Names[0].Repo, g.Images[0].BuiltBy
		}
		if image != c.image || by != c.builtBy || g.JavaVersion != c.java || g.Executable != c.exec {
			t.Errorf("%s: image %q by %q java %q exec %v", c.name, image, by, g.JavaVersion, g.Executable)
		}
	}
}
