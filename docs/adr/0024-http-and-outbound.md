# 0024. HTTP routes, contracts and outbound connections

- **Status**: Accepted
- **Date**: 2026-10-29

## Context and Problem Statement

Most codebases Archstats reads are HTTP APIs, and the questions an architect brings to them are HTTP questions: which routes does this system offer, what does each one reach, does the code match the contract the team publishes, and what does the system call out to, configured where, per environment. A technical baseline assessment (the RVO/DICTU "nulmeting" checklist) asks the same things as evidence: data flows, interface contracts, failure handling on links, access, the NLGov API design rules, chain dependencies.

Revision 13 already read routes (`entry_points`) and data access, but stopped at the handler, ignored contracts, and dropped declarative clients on the floor. CI/CD (revision 16) moved the service-to-service map out of the deploy files. Nothing read where the code goes out.

## Decision Drivers

- **Evidence over verdicts**: every row carries its file and line, and a call says how sure it is (`declared`, `resolved`, `partial`, `unresolved`) instead of guessing.
- **Every major framework, not one**: Spring, JAX-RS, Micronaut, ASP.NET (attributes, conventional, minimal), NestJS, Express and friends, FastAPI, Flask, Django, Gin/Echo/Chi/net/http, Laravel, Symfony, Ktor.
- **Offline and lexical**: no compiler, no running service; the same scan on a laptop.
- **No secrets**: configuration values are kept only when they name a place, without user or password.
- **Scan cost**: a large repository (fineract, 817 routes) may not get noticeably slower.

## Considered Options

- **Class-level reach through the unit graph**: cheap, already there, but a controller with twenty routes gives all twenty the same footprint, which is the question unanswered.
- **A real call graph from the language packs**: precise for Java and C#, but eleven packs would each need call resolution, and dynamic languages would still fall back to guessing.
- **A lexical member-level trace in the navigation extension**: per file, the declared type of every field, property, constructor parameter and constructed variable, plus every call site; a call `orders.find(id)` resolves through `orders`' declared type, an import, a static type name or a bare function name, and an interface to its implementations through `unit_supertypes` and `bindings`. An untyped local is counted as unknown, never guessed.

## Decision Outcome

Chosen option: "a lexical member-level trace in the navigation extension", because it answers per route, works the same way in every language with a route reader, and says how much of a route it could follow (`route_reach.unknown`).

The same extension reads:

- **Contracts**: OpenAPI 3 and Swagger 2 documents, matched to routes by method and path template under any server base path and the application's context path; a generator in a POM, Gradle build, `openapitools.json`, NSwag config or `.csproj` says whether a contract is served or consumed. Calls through a client generated from a consumed contract are found by `operationId`.
- **Design rules**: the NLGov rules that can be read from a document or a path as written, named after the Logius Spectral ruleset.
- **Access**: marks on the handler and its class, middleware groups, Django REST framework's `permission_classes`, and Spring Security matchers scoped by their filter chain's `securityMatcher`.
- **Outbound calls**: declared clients (Feign, MicroProfile, `@HttpExchange`, Retrofit, Refit), imperative HTTP clients gated on the receiver's type or name, messaging, databases, caches, search, mail, LDAP, object storage, file transfer, gRPC, GraphQL and SOAP. Addresses are rendered from literals, constants, `@Value`, `@ConfigurationProperties`, .NET options and `IConfiguration`, environment variables, Laravel's `config()`/`env()` and Django's `settings`, then resolved against the configuration files per environment.
- **Configuration**: Spring, .NET, `.env`, compose, Helm values, Kubernetes manifests, Laravel and Django settings, and node-config files, keeping only places.

### Consequences

- **Good**: a route reads as the chain from handler to tables and systems; a call to a route or a consumer in the same snapshot links to it, so a frontend's `fetch('/api/orders')` and two scanned services join.
- **Good**: the snapshot answers several nulmeting questions with file-and-line evidence: 2.02, 2.03, 3.04 in part, 4.04, 5.09.
- **Bad**: lexical resolution misses dynamic dispatch through maps, reflection and callbacks, and an untyped variable in JavaScript or Python is often unknown. `route_reach` makes that visible rather than hiding it.
- **Bad**: on fineract the scan takes about 12% longer (26s against 23s), most of it reading call sites and declared types.
