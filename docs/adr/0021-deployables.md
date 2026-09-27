# 0021. Deployables

- **Status**: Accepted
- **Date**: 2026-09-27

## Context and Problem Statement

A component is where the code says it lives (ADR 0007) and a module is what the project builds and publishes (ADR 0017). Neither says what gets built and shipped as one unit, or how those units depend on each other at run time, and for a system of services that is the architecture.

Measured before this was written:

- A 26-repository enterprise workspace reads as 26 islands: the Java imports end at each repository's edge. Its deployment lives in 74 per-environment Helm values files, every pipeline hands its work to one central workflow repository at `@main`, and the only record of which service calls which is the hosts in each service's Spring configuration.
- Eleven public reference systems joined code to image to workload at 85–99% where the repository describes the running system (microservices-demo, the OTel demo, bank-of-anthos, spring-petclinic-microservices, eShop), and gave the real call graph from configuration in four of five. Product repositories (Mattermost, Kafka) deploy from elsewhere and yield little.
- Every one of those joins needed one format-specific step, and skipped it failed outright: compose images named in `.env` (0 of 34 joined without it), a Maven image plugin configured in a parent pom's profile (0 of 9), Jib modules named by path in Skaffold and by artifactId in the pom, a ConfigMap loaded with `envFrom`, a job matrix of maps.

The files were already walked, and ignored.

## Decision Drivers

- **Evidence over verdicts**: the engine records what the files state, with file and line, and says how every join was made. A judgement about the running system is the architect's.
- **Offline**: no cluster, cloud account, registry or remote chart is consulted; a workspace is scanned the same way on a laptop with no network.
- **Few formats, done properly**: a shallow read of a format is worth close to nothing.
- **Small binary**: this ships in a desktop app.

## Considered Options

- **Architectural quanta**: have the engine name quanta, deployables joined by synchronous calls and shared data. Rejected: the definition rests on run-time coupling the files only hint at, and it names a conclusion rather than evidence, the mistake the lens readings were renamed to avoid.
- **A general system graph**: every artifact, pipeline, workload and cloud resource as typed nodes. Rejected as a second product; Terraform, CI security and delivery metrics answer questions other tools answer well.
- **Deployables and the evidence between them**: what is built and shipped, what goes into it, how it is built, where it runs, what it talks to and what technology it carries.

## Decision Outcome

Chosen option: "Deployables and the evidence between them", in `extensions/deployables`, because it is what the files state, it is the part no offline tool gives, and it is where multi-repository workspaces are blind today. Quanta are groups the architect draws, proposed by a lens reading that names the links it merged on.

- A `FileAnalyzer` keeps the content of files with a `system_kind` (and Aspire app hosts and CycloneDX files); readers turn each into facts; one linker joins facts across files after aggregation, using the module map and the file-to-component index.
- Every join carries a `resolution`. Names are normalised hard (registry, tag, digest and interpolated segments dropped), matched exactly, and a tie is refused into `deployable_unresolved` rather than chosen between.
- Secrets: a key that names a secret is stored without its value; URLs keep scheme, host, port and database only.
- YAML is read with `gopkg.in/yaml.v3` nodes so every fact keeps its line. The Dockerfile reader is hand-written: BuildKit's parser pulls in the protobuf runtime (about 3.6 MB of binary) and a newer Go.
- Helm templates are not rendered. Kustomize, Jenkins scripted pipelines, Gradle scripts, CDK and Pulumi are read only as far as their literal shape allows; Terraform is not read.

### Consequences

- **Good**: a multi-repository workspace connects: the enterprise workspace's gateway routes to 16 services, three services share one database and one topic, and one internal library sits inside 15 services.
- **Good**: the snapshot answers "which code ends up in which image", "what builds and deploys it", "in which environments", and "on which runtime and framework".
- **Bad**: name joins can still be wrong where two things are named alike in different places; they are labelled as the weakest evidence and shown dashed.
- **Bad**: a ConfigMap shared by several services hands each one every address in it; such links are labelled `shared_config`.
- **Bad**: a workspace whose own ignore file hides non-source files (`*.*`, `!*.java`) yields no deployables until that is changed.
