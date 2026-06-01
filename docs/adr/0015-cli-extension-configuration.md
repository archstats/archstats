# 0015. Dynamic CLI Configurable Extensions

- **Status**: Accepted
- **Date**: 2026-06-01

## Context and Problem Statement

Because `archstats` utilizes a microkernel style architecture, extensions are often decoupled from the command line execution code. However, extensions frequently require custom input parameters (e.g. thresholds, custom path ignores, or Git commit history boundaries) from the CLI user. Hardcoding CLI flags in Cobra for every possible extension breaks pluggability.

## Decision Drivers

- **Extensibility**: Extensions must be able to introduce their own CLI arguments dynamically.
- **Robustness**: Type-safe command arguments with clean validation.
- **Modularity**: The command-line parsing layer should remain decoupled from specific extension configurations.

## Considered Options

- **Cobra-Level Hardcoding**: Define every extension flag explicitly inside the `cmd/` package. Fails whenever third-party extensions are loaded.
- **Dynamic CLI Configurable Extensions**: Extensions implement configuration interfaces allowing them to register custom CLI flags dynamically into Cobra and receive their typed values before execution.

## Decision Outcome

Chosen option: **Dynamic CLI Configurable Extensions** (implemented in `cmd/config/cli_config_extension.go`).

We expose standard configuration interfaces. When Cobra initializes, it traverses all active extensions. Extensions implementing these configuration interfaces register custom flags directly onto the Cobra command structure. Before execution, the parsed arguments are bound and passed into the extensions.

### Consequences

- **Good**: True architectural decoupling. Adding or removing an extension automatically handles registering and documenting its respective command-line flags.
- **Good**: Users can easily inspect extension flags via `--help`.
- **Bad**: Order of extension loading and flag registration is critical to prevent name collisions across separate extensions.
