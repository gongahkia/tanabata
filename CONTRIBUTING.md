# Contributing

Tanabata accepts focused changes with tests and documentation proportionate to their impact. By participating, you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## Local setup

Install the pinned toolchain:

```console
$ mise trust
$ mise install
```

If you use asdf:

```console
$ asdf install
```

Install and run pre-commit hooks:

```console
$ pre-commit install
$ pre-commit run --all-files
```

## Development

Run backend tests with `make test`. Use `make ingest` to rebuild local catalog data and `make run` to start the API. Validate container-facing changes with `./scripts/e2e-endpoint-sweep.sh`.

## Changes

Use Conventional Commit subjects, such as `feat(api): add source endpoint` or `docs(adr): explain catalog policy`. Keep commits limited to one behavioral change where practical.

Pull requests should state the user-visible effect, include tests for changed behavior, update OpenAPI and generated API docs for public endpoint changes, and add an ADR for consequential architectural decisions. Include screenshots only for visual changes.
