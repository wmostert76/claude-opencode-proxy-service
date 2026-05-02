# Claude Go — Development Notes

## Release Flow

Pushing to `main` triggers `.github/workflows/release.yml`:

1. **Reads `VERSION`** (e.g. `1.1.0`)
2. **Builds** cross-platform binaries (linux/darwin/windows, amd64/arm64)
3. **Creates a GitHub release** tagged `v1.1.0-{shortsha}` with all binaries attached
4. **Skips** if a release for the same base version already exists
5. **Bumps `VERSION`** minor by 1 (e.g. `1.1.0` → `1.2.0`), commits with `[skip ci]`, pushes back

The bump commit includes `[skip ci]` so it does **not** trigger another release.

End-users update via:
```
claude-go update
```
This downloads the **latest** release binary from GitHub.

## Building Locally

```
go build -ldflags="-s -w -X main.version=$(cat VERSION)" -o claude-go ./cmd/claude-go/
```

## Adding Dependencies

Module path: `github.com/wmostert76/claude-go`
No external dependencies beyond stdlib.
