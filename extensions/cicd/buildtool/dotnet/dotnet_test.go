package dotnet

import (
	"reflect"
	"testing"
)

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
