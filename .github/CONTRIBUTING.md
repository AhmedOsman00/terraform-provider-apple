# Contributing

Thanks for taking the time. This is a small, single-maintainer provider, so the
process is light — but a few things about it are unusual enough to be worth
reading before you write code.

By participating you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).
Security problems go through [SECURITY.md](SECURITY.md), not the issue tracker.

## Getting set up

```shell
git clone https://github.com/AhmedOsman00/terraform-provider-apple
cd terraform-provider-apple
make build
make test        # no credentials needed
```

You need Go >= 1.25, and a `terraform` binary on your PATH for `make generate`
and `make validate-examples`. `golangci-lint` v2.13 or later is needed for
`make lint` — v9 of the config format is what `.golangci.yml` uses.

`make` on its own runs `fmt lint install generate`, which is the same set CI
checks.

To run your working tree against a real Apple team, wire the built binary up
through a `dev_overrides` block in `~/.terraformrc`; the
[getting started guide](../docs/guides/getting-started.md#installing-the-provider)
has the block. Note that `dev_overrides` makes `terraform init` refuse to run,
which is why `make validate-examples` uses a filesystem mirror instead.

## Reporting a bug

Include the provider version, the Terraform version, the configuration that
reproduces it with credentials redacted, and the actual output of
`TF_LOG=DEBUG terraform apply` around the failure. For anything that reaches
Apple, the API error text matters: the provider maps a lot of Apple's responses
into friendlier diagnostics, and knowing which one you hit narrows it fast.

Apple's API is the source of most surprises here. If a resource behaves oddly,
it is worth checking whether App Store Connect's own web UI can do what you are
asking — several attributes are write-once or unreadable purely because Apple
publishes no endpoint for them, and those are documented per-resource in
[CLAUDE.md](../CLAUDE.md).

## Making a change

Four things are easy to miss.

**Register new resources and data sources by hand.** Nothing is discovered
automatically — a new constructor must be added to both `DataSources()` and
`Resources()` in `internal/provider/provider.go` or it simply will not exist.

**Keep the two layers separate.** `internal/apple/` is the App Store Connect
client: plain `Get*/Create*/Update*/Delete*` methods returning `models.*`
structs, with no Terraform types anywhere in it. `internal/provider/` is the
Terraform layer, one package per domain, following the four-file convention
(`models.go`, `resource.go`, `data_source.go`, `filters.go`). Match the shape of
the package you are working in.

**Every list endpoint must paginate.** Apple has almost no server-side
filtering, so the "get by identifier" helpers list a whole collection and scan
it client-side. Reading only the first page silently truncates the result and
makes those helpers wrong. Go through `getAllPages` in
`internal/apple/pagination.go`.

**Regenerate the docs.** `docs/` is built by `tfplugindocs` from the
`MarkdownDescription` strings on each schema plus the matching directory under
`examples/`, and CI fails the `generate` job on any diff. Run `make generate`
and commit the result whenever you touch a schema, an example, or a guide.
Never hand-edit anything under `docs/` — the whole directory is deleted and
re-rendered on every run. Hand-written prose belongs in `templates/`.

Then run, at minimum:

```shell
make fmt lint
make test
make generate           # commit the diff
make validate-examples  # only check that catches an example the schema rejects
```

## Style

The existing code is the specification, but a few conventions are load-bearing:

- `MarkdownDescription`, never `Description` — it is what the generated docs are
  built from, so enumerate the valid enum values in the string.
- Immutable Apple attributes get `stringplanmodifier.RequiresReplace()`;
  Apple-computed ones get `stringplanmodifier.UseStateForUnknown()`.
- Error handling is deliberately verbose. Map "already exists", "not found" and
  404 into specific, actionable diagnostics rather than surfacing a raw HTTP
  error.
- Customer-facing strings are validated in **characters**, not bytes:
  `stringvalidator.UTF8LengthBetween`. Apple's 30- and 45-character limits are
  character counts, and `LengthBetween` counts bytes — which rejected a legal
  27-character Arabic description at "49".
- Log with `tflog` and `tflog.SetField`. Never log a credential value.
- Data sources have no `last_updated` attribute, on purpose. A data source is
  re-read every plan with no prior state, so the value can only ever be "now",
  and it propagates a perpetual diff to everything referencing it.
- Every `.go` file carries the copywrite header. `make generate` stamps new
  files for you.

## Tests

Two tiers, and the distinction matters:

**Credential-free** tests run on every pull request. `httptest`-backed client
tests and pure-function tests over the filter and model code. `apple.Client`
has all-exported fields, so pointing one at a test server needs no production
seam:

```go
&apple.Client{HostURL: srv.URL, HTTPClient: srv.Client(), Token: "test"}
```

New filter or conversion logic should land with a table-driven test here. This
is the tier a contributor without an Apple Developer account can fully exercise.

**Acceptance** tests hit the real App Store Connect API and create billable
resources in a real team. They skip themselves without `TF_ACC=1` and
credentials, and they run only on `workflow_dispatch`. Read
[ACCEPTANCE_TESTING.md](../ACCEPTANCE_TESTING.md) before running them — it lists
what every test creates, what it leaves behind, and the cleanup list for an
interrupted run. Three constraints on anything you add there:

- **Do not register devices.** Apple has no device delete, so those tests
  consume team slots permanently and pass exactly once per team, ever. The
  existing two are the only ones there should be; profile tests use
  `IOS_APP_STORE`, which needs none.
- **Randomise product identifiers** for anything under App Store Connect.
  Apple never releases a product ID it has seen, so a fixed one passes once per
  account and fails forever after. Use the `testAcc*ProductID` helpers.
- **Check against Apple, not against state.** A `CheckDestroy` that only
  inspects Terraform state passes while the resource is still live in the
  portal. Go through `testAccAPIClient()`.

You do not need to run the acceptance tier to contribute. Say in the pull
request which tiers you ran, and the maintainer will run the rest.

## Pull requests

Keep them focused, and update `CHANGELOG.md` under the unreleased heading using
the existing categories (`FEATURES`, `BUG FIXES`, `BREAKING CHANGES`,
`DOCUMENTATION`, `NOTES`). Entries there are prose, not commit subjects — say
what changed and why it matters to someone writing configuration.

Removing or renaming an attribute on a working resource is breaking and needs a
deprecation cycle. There is one precedent in the history that skipped it, on a
resource that had never successfully created anything; that escape hatch does
not generalise.

## Licence

Contributions are accepted under [MPL-2.0](../LICENSE), matching the project.
