package deployables

import (
	"reflect"
	"testing"
)

// The spring-petclinic-microservices shape: the root holds the properties and
// the exec-maven-plugin configuration in a profile's pluginManagement; each
// module declares the plugin in the same profile and overrides the
// Dockerfile directory.
const petclinicRoot = `<?xml version="1.0" encoding="UTF-8"?>
<project>
  <parent>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-starter-parent</artifactId>
    <version>3.4.1</version>
    <relativePath/>
  </parent>
  <groupId>org.springframework.samples</groupId>
  <artifactId>spring-petclinic-microservices</artifactId>
  <version>4.0.1</version>
  <packaging>pom</packaging>
  <modules>
    <module>spring-petclinic-customers-service</module>
  </modules>
  <properties>
    <java.version>17</java.version>
    <docker.image.prefix>springcommunity</docker.image.prefix>
    <docker.image.dockerfile.dir>${basedir}</docker.image.dockerfile.dir>
    <spring-cloud.version>2024.0.0</spring-cloud.version>
  </properties>
  <dependencyManagement>
    <dependencies>
      <dependency>
        <groupId>org.springframework.cloud</groupId>
        <artifactId>spring-cloud-dependencies</artifactId>
        <version>${spring-cloud.version}</version>
        <type>pom</type>
        <scope>import</scope>
      </dependency>
    </dependencies>
  </dependencyManagement>
  <profiles>
    <profile>
      <id>buildDocker</id>
      <build>
        <pluginManagement>
          <plugins>
            <plugin>
              <groupId>org.codehaus.mojo</groupId>
              <artifactId>exec-maven-plugin</artifactId>
              <executions>
                <execution>
                  <configuration>
                    <executable>${container.executable}</executable>
                    <workingDirectory>${docker.image.dockerfile.dir}</workingDirectory>
                    <arguments>
                      <argument>build</argument>
                      <argument>-f</argument>
                      <argument>Dockerfile</argument>
                      <argument>--build-arg</argument>
                      <argument>ARTIFACT_NAME=${project.build.finalName}</argument>
                      <argument>-t</argument>
                      <argument>${docker.image.prefix}/${project.artifactId}</argument>
                      <argument>${project.build.directory}</argument>
                    </arguments>
                  </configuration>
                </execution>
              </executions>
            </plugin>
          </plugins>
        </pluginManagement>
      </build>
      <properties>
        <container.executable>docker</container.executable>
      </properties>
    </profile>
  </profiles>
</project>`

const petclinicCustomers = `<?xml version="1.0" encoding="UTF-8"?>
<project>
  <artifactId>spring-petclinic-customers-service</artifactId>
  <packaging>jar</packaging>
  <parent>
    <groupId>org.springframework.samples</groupId>
    <artifactId>spring-petclinic-microservices</artifactId>
    <version>4.0.1</version>
  </parent>
  <properties>
    <docker.image.dockerfile.dir>${basedir}/../docker</docker.image.dockerfile.dir>
  </properties>
  <dependencies>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-web</artifactId>
    </dependency>
    <dependency>
      <groupId>org.springframework.cloud</groupId>
      <artifactId>spring-cloud-starter-config</artifactId>
    </dependency>
  </dependencies>
  <build>
    <plugins>
      <plugin>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-maven-plugin</artifactId>
      </plugin>
    </plugins>
  </build>
  <profiles>
    <profile>
      <id>buildDocker</id>
      <build>
        <plugins>
          <plugin>
            <groupId>org.codehaus.mojo</groupId>
            <artifactId>exec-maven-plugin</artifactId>
          </plugin>
        </plugins>
      </build>
    </profile>
  </profiles>
</project>`

func TestPomPetclinicImageFromInheritedExecPlugin(t *testing.T) {
	poms := readPoms(map[string][]byte{
		"pom.xml": []byte(petclinicRoot),
		"spring-petclinic-customers-service/pom.xml": []byte(petclinicCustomers),
	})
	if len(poms) != 2 {
		t.Fatalf("poms = %d", len(poms))
	}
	var root, cust *pom
	for _, p := range poms {
		if p.ArtifactID == "spring-petclinic-microservices" {
			root = p
		} else {
			cust = p
		}
	}
	if cust.parent != root {
		t.Fatal("the module's parent is the root pom")
	}
	if len(root.Images) != 0 {
		t.Errorf("the root only manages the plugin; it builds no image: %+v", root.Images)
	}
	if len(cust.Images) != 1 {
		t.Fatalf("customers images = %+v", cust.Images)
	}
	b := cust.Images[0]
	if b.Names[0].Repo != "spring-petclinic-customers-service" || b.BuiltBy != "maven-docker" {
		t.Errorf("image = %+v", b)
	}
	if b.Dockerfile != "docker/Dockerfile" || b.Context != "spring-petclinic-customers-service/target" {
		t.Errorf("dockerfile %q context %q", b.Dockerfile, b.Context)
	}
	if b.File != "spring-petclinic-customers-service/pom.xml" || b.Line == 0 {
		t.Errorf("evidence %s:%d", b.File, b.Line)
	}
	if !cust.Executable || root.Executable {
		t.Errorf("executable: customers %v root %v", cust.Executable, root.Executable)
	}
	if cust.SpringBoot != "3.4.1" || cust.JavaVersion != "17" {
		t.Errorf("spring boot %q java %q", cust.SpringBoot, cust.JavaVersion)
	}
	if cust.Props["project.build.finalName"] != "spring-petclinic-customers-service-4.0.1" {
		t.Errorf("finalName = %q", cust.Props["project.build.finalName"])
	}
}

func TestPomJibAndSpringBootImages(t *testing.T) {
	poms := readPoms(map[string][]byte{
		"ledger/pom.xml": []byte(`<project><groupId>anthos</groupId><artifactId>ledgerwriter</artifactId><version>1</version>
<build><plugins><plugin><groupId>com.google.cloud.tools</groupId><artifactId>jib-maven-plugin</artifactId>
<configuration><from><image>eclipse-temurin:21</image></from><to><image>gcr.io/proj/ledgerwriter:v1</image></to></configuration></plugin></plugins></build></project>`),
		"jibdefault/pom.xml": []byte(`<project><artifactId>balancereader</artifactId>
<build><plugins><plugin><artifactId>jib-maven-plugin</artifactId></plugin></plugins></build></project>`),
		"boot/pom.xml": []byte(`<project><artifactId>orders</artifactId>
<build><plugins><plugin><artifactId>spring-boot-maven-plugin</artifactId>
<configuration><image><builder>paketobuildpacks/builder</builder><name>registry.acme.io/shop/orders:${project.version}</name></image></configuration></plugin></plugins></build></project>`),
		"plainboot/pom.xml": []byte(`<project><artifactId>billing</artifactId>
<build><plugins><plugin><artifactId>spring-boot-maven-plugin</artifactId></plugin></plugins></build></project>`),
		"lib/pom.xml": []byte(`<project><artifactId>common</artifactId><packaging>jar</packaging></project>`),
	})
	got := map[string][]string{}
	exec := map[string]bool{}
	for _, p := range poms {
		for _, b := range p.Images {
			got[p.ArtifactID] = append(got[p.ArtifactID], b.BuiltBy+":"+b.Names[0].Repo)
		}
		exec[p.ArtifactID] = p.Executable
	}
	want := map[string][]string{
		"ledgerwriter":  {"jib:ledgerwriter"},
		"balancereader": {"jib:balancereader"},
		"orders":        {"spring-boot:orders"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("images = %v\nwant %v", got, want)
	}
	if !exec["orders"] || !exec["billing"] || exec["common"] {
		t.Errorf("executables = %v", exec)
	}
}

func TestPomFabric8AndSpotify(t *testing.T) {
	poms := readPoms(map[string][]byte{
		"a/pom.xml": []byte(`<project><artifactId>a</artifactId><build><plugins><plugin><groupId>io.fabric8</groupId><artifactId>docker-maven-plugin</artifactId>
<configuration><images><image><name>acme/a-service:%l</name><build><dockerFile>Dockerfile</dockerFile></build></image></images></configuration></plugin></plugins></build></project>`),
		"b/pom.xml": []byte(`<project><artifactId>b</artifactId><build><plugins><plugin><artifactId>dockerfile-maven-plugin</artifactId>
<configuration><repository>acme/b-service</repository><tag>${project.version}</tag></configuration></plugin></plugins></build></project>`),
	})
	for _, p := range poms {
		if len(p.Images) != 1 {
			t.Errorf("%s: images %+v", p.ArtifactID, p.Images)
			continue
		}
		want := map[string]string{"a": "a-service", "b": "b-service"}[p.ArtifactID]
		if p.Images[0].Names[0].Repo != want {
			t.Errorf("%s: %q", p.ArtifactID, p.Images[0].Names[0].Repo)
		}
	}
}

func TestPomExecPluginThatIsNotDocker(t *testing.T) {
	poms := readPoms(map[string][]byte{
		"pom.xml": []byte(`<project><artifactId>x</artifactId><build><plugins><plugin><artifactId>exec-maven-plugin</artifactId>
<configuration><executable>npm</executable><arguments><argument>run</argument><argument>build</argument></arguments></configuration></plugin></plugins></build></project>`),
	})
	if len(poms[0].Images) != 0 {
		t.Errorf("npm run build is not an image: %+v", poms[0].Images)
	}
}

func TestPomDependenciesAndManagedVersions(t *testing.T) {
	poms := readPoms(map[string][]byte{
		"pom.xml": []byte(`<project><groupId>g</groupId><artifactId>parent</artifactId><version>1</version><packaging>pom</packaging>
<properties><common.version>2.1.0</common.version></properties>
<dependencyManagement><dependencies><dependency><groupId>g</groupId><artifactId>qp-common</artifactId><version>${common.version}</version></dependency></dependencies></dependencyManagement></project>`),
		"svc/pom.xml": []byte(`<project><parent><groupId>g</groupId><artifactId>parent</artifactId><version>1</version></parent><artifactId>svc</artifactId>
<dependencies>
  <dependency><groupId>g</groupId><artifactId>qp-common</artifactId></dependency>
  <dependency><groupId>org.x</groupId><artifactId>lib</artifactId><version>${missing.version}</version></dependency>
  <dependency><groupId>junit</groupId><artifactId>junit</artifactId><version>4.13</version><scope>test</scope></dependency>
</dependencies></project>`),
	})
	var svc *pom
	for _, p := range poms {
		if p.ArtifactID == "svc" {
			svc = p
		}
	}
	got := map[string]string{}
	for _, d := range svc.Deps {
		got[d.ArtifactID] = d.Version + "|" + d.Scope
		if d.Line == 0 {
			t.Errorf("%s has no line", d.ArtifactID)
		}
	}
	want := map[string]string{"qp-common": "2.1.0|", "lib": "|", "junit": "4.13|test"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("deps = %v", got)
	}
}

func TestPomSpringBootFromBomImport(t *testing.T) {
	poms := readPoms(map[string][]byte{
		"pom.xml": []byte(`<project><artifactId>app</artifactId><properties><sb.version>2.3.3.RELEASE</sb.version><maven.compiler.source>1.8</maven.compiler.source></properties>
<dependencyManagement><dependencies><dependency><groupId>org.springframework.boot</groupId><artifactId>spring-boot-dependencies</artifactId><version>${sb.version}</version><type>pom</type><scope>import</scope></dependency></dependencies></dependencyManagement></project>`),
	})
	if poms[0].SpringBoot != "2.3.3.RELEASE" || poms[0].JavaVersion != "8" {
		t.Errorf("spring boot %q java %q", poms[0].SpringBoot, poms[0].JavaVersion)
	}
}

func TestPomParentByCoordinatesWhenNotBeside(t *testing.T) {
	poms := readPoms(map[string][]byte{
		"build/parent/pom.xml": []byte(`<project><groupId>g</groupId><artifactId>corp-parent</artifactId><version>1</version><properties><java.version>11</java.version></properties></project>`),
		"svc/pom.xml":          []byte(`<project><parent><groupId>g</groupId><artifactId>corp-parent</artifactId><version>1</version></parent><artifactId>svc</artifactId></project>`),
	})
	for _, p := range poms {
		if p.ArtifactID == "svc" && (p.parent == nil || p.JavaVersion != "11") {
			t.Errorf("parent %v java %q", p.parent, p.JavaVersion)
		}
	}
}

func TestPomMalformedAndCyclicParents(t *testing.T) {
	poms := readPoms(map[string][]byte{
		"bad/pom.xml": []byte(`<project><artifactId>unterminated`),
		"a/pom.xml":   []byte(`<project><parent><artifactId>b</artifactId></parent><artifactId>a</artifactId></project>`),
		"b/pom.xml":   []byte(`<project><parent><artifactId>a</artifactId></parent><artifactId>b</artifactId></project>`),
	})
	if len(poms) != 2 {
		t.Errorf("a malformed pom is skipped: %d", len(poms))
	}
	for _, p := range poms {
		if len(p.ancestors()) > 1 {
			t.Errorf("a cycle must stop: %d ancestors", len(p.ancestors()))
		}
	}
}

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

func TestCsproj(t *testing.T) {
	web := readCsproj("src/Catalog.API/Catalog.API.csproj", []byte(`<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup><TargetFramework>net9.0</TargetFramework><ContainerRepository>eshop/catalog-api</ContainerRepository></PropertyGroup>
  <ItemGroup><PackageReference Include="Asp.Versioning.Http" Version="8.1.0" /><PackageReference Include="Npgsql" /></ItemGroup>
</Project>`))
	if !web.Web || web.Name != "Catalog.API" || web.TargetFramework != "net9.0" {
		t.Errorf("web = %+v", web)
	}
	if len(web.Images) != 1 || web.Images[0].Names[0].Repo != "catalog-api" || web.Images[0].BuiltBy != "dotnet-publish" {
		t.Errorf("images = %+v", web.Images)
	}
	if len(web.Packages) != 2 || web.Packages[0].Version != "8.1.0" || web.Packages[1].Version != "" {
		t.Errorf("packages = %+v", web.Packages)
	}
	lib := readCsproj("src/Shared/Shared.csproj", []byte(`<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFrameworks>net8.0;net9.0</TargetFrameworks></PropertyGroup></Project>`))
	if lib.Web || lib.TargetFramework != "net8.0" {
		t.Errorf("lib = %+v", lib)
	}
	host := readCsproj("src/eShop.AppHost/eShop.AppHost.csproj", []byte(`<Project Sdk="Microsoft.NET.Sdk"><Sdk Name="Aspire.AppHost.Sdk" Version="9.0.0" /></Project>`))
	if !host.AspireHost {
		t.Error("an Aspire app host is recognised")
	}
}

const eShopAppHost = `using eShop.AppHost;

var builder = DistributedApplication.CreateBuilder(args);

builder.AddForwardedHeaders();

var redis = builder.AddRedis("redis");
var rabbitMq = builder.AddRabbitMQ("eventbus")
    .WithLifetime(ContainerLifetime.Persistent);
var postgres = builder.AddPostgres("postgres")
    .WithImage("ankane/pgvector");

var catalogDb = postgres.AddDatabase("catalogdb");
var identityDb = postgres.AddDatabase("identitydb");

// var commented = builder.AddProject<Projects.Nope>("nope");

var identityApi = builder.AddProject<Projects.Identity_API>("identity-api", launchProfileName)
    .WithReference(identityDb);

var identityEndpoint = identityApi.GetEndpoint(launchProfileName);

var basketApi = builder.AddProject<Projects.Basket_API>("basket-api")
    .WithReference(redis)
    .WithReference(rabbitMq).WaitFor(rabbitMq)
    .WithEnvironment("Identity__Url", identityEndpoint);

builder.AddProject<Projects.OrderProcessor>("order-processor")
    .WithReference(rabbitMq).WaitFor(rabbitMq);

var webApp = builder.AddProject<Projects.WebApp>("webapp", launchProfileName)
    .WithReference(basketApi)
    .WithReference(catalogDb);

builder.Build().Run();
`

func TestAspireHost(t *testing.T) {
	rs := readAspireHost("src/eShop.AppHost/Program.cs", []byte(eShopAppHost))
	byName := map[string]*aspireResource{}
	for _, r := range rs {
		byName[r.Name] = r
	}
	if _, ok := byName["nope"]; ok {
		t.Error("commented-out code is not read")
	}
	for name, kind := range map[string]string{"redis": "Redis", "eventbus": "RabbitMQ", "postgres": "Postgres", "catalogdb": "Database", "identity-api": "Project", "basket-api": "Project", "order-processor": "Project", "webapp": "Project"} {
		if byName[name] == nil || byName[name].Kind != kind {
			t.Errorf("%s: %+v", name, byName[name])
		}
	}
	if byName["catalogdb"].Parent != "postgres" {
		t.Errorf("catalogdb parent = %q", byName["catalogdb"].Parent)
	}
	if byName["basket-api"].Project != "Basket.API" || byName["order-processor"].Var != "" {
		t.Errorf("basket %+v order %+v", byName["basket-api"], byName["order-processor"])
	}
	refs := func(name string) []string {
		var out []string
		for _, r := range byName[name].Refs {
			out = append(out, r.Var)
		}
		return out
	}
	if got := refs("basket-api"); !reflect.DeepEqual(got, []string{"redis", "rabbitMq", "identityApi"}) {
		t.Errorf("basket refs = %v (the endpoint resolves to its project; WaitFor is not a reference)", got)
	}
	if got := refs("webapp"); !reflect.DeepEqual(got, []string{"basketApi", "catalogDb"}) {
		t.Errorf("webapp refs = %v", got)
	}
	if byName["identity-api"].Line != 18 || byName["basket-api"].Refs[0].Line != 24 {
		t.Errorf("lines: identity %d basket ref %d", byName["identity-api"].Line, byName["basket-api"].Refs[0].Line)
	}
}
