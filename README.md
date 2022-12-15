# crossplane-lint
[Crossplane compositions](https://docs.crossplane.io/v1.10/reference/composition/) are a great way to build platform APIs. However they are also a great way to introduce bugs! `crossplane-lint` helps to find those issues as it knows about the internals of compositions & XRDs.

It validates:
- Schema of managed resources against compositions
- Schema of XRDs against compositions
- Different patch types (`FromCompositeFieldPath`, `ToCompositeFieldPath` and `CombineFromComposite`)

## Commands

For a detailed overview of all commands and parameters run `crossplane-lint --help`.

### Package linting

```bash
crossplane-lint package -f <package-dir>
```

Scans crossplane composition and XRDs in the given directory for issues.
The linter can load additional packages (for example to include provider CRDs) that are defined in `.crossplane-lint.yaml` in the current working directory:

```yaml
additionalPackages:
  - image: crossplanecontrib/provider-aws:v0.34.0
  - image: crossplanecontrib/provider-gitlab:v0.3.0
  - image: crossplanecontrib/provider-helm:v0.12.0
  - image: xpkg.upbound.io/grafana/provider-grafana:v0.0.10
  - image: crossplanecontrib/provider-kubernetes:v0.5.0
  - image: crossplanecontrib/provider-styra:v0.3.0
```
## Roadmap
- Patch Static Type Checking
- Patch Transform validation
- Validaton of the XRD Schema itself
- Disable linter rules per file / line
## Development

The binaries are built using [goreleaser](https://github.com/goreleaser/goreleaser).
The individual build steps are executed using `make`.

To build a snapshot for your current OS:
```
make build
```

To build a snapshot for all platforms:
```
make build.all
```

To lint your code:
```
make lint
```

Currently, only builds for Mac and Linux (arm and amd64) are available.
