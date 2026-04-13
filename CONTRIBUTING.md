# Contributing to kubernetes-graphql-gateway

## General Remarks

You are welcome to contribute content (code, documentation etc.) to this open source project.

There are some important things to know:

1. You must **comply to the license of this project**, **accept the Developer Certificate of Origin** (see below) before being able to contribute. The acknowledgement to the DCO will usually be requested from you as part of your first pull request to this project.
2. Please **adhere to our [Code of Conduct](https://github.com/platform-mesh/.github/blob/main/CODE_OF_CONDUCT.md)**.
3. If you plan to use **generative AI for your contribution**, please see our [guideline for AI-generated code contributions](https://github.com/platform-mesh/.github/blob/main/CONTRIBUTING_USING_GENAI.md).
4. **Not all proposed contributions can be accepted**. Some features may fit another project better or don't fit the general direction of this project. Of course, this doesn't apply to most bug fixes, but a major feature implementation for instance needs to be discussed with one of the maintainers first. The best way would be to just open an issue to discuss the feature you plan to implement (make it clear that you intend to contribute).

## Developer Certificate of Origin (DCO)

Contributors will be asked to accept a DCO before they submit the first pull request to this project, this happens in an automated fashion during the submission process. SAP uses [the standard DCO text of the Linux Foundation](https://developercertificate.org/).

## How to Contribute

1. Make sure the change is welcome (see [General Remarks](#general-remarks)).
2. Fork the repository and create a branch.
3. Make your changes and verify them locally:
   - Run `task validate` to run code generation, linting, and all tests.
   - See [Development](#development) below for all available task commands.
4. Commit using [Conventional Commits](https://www.conventionalcommits.org/) format (e.g. `feat:`, `fix:`, `chore:`).
5. Create a pull request.
6. Follow the link posted by the CLA assistant to your pull request and accept it, as described above.
7. Wait for code review and approval, possibly enhancing your change on request.

## Development

Prerequisites: Go (see version in `go.mod`), Docker, kubectl, [Task](https://taskfile.dev).

Key commands:

- `task validate` — full validation (generate + lint + test)
- `task test` — unit tests with coverage
- `task generate` — regenerate CRDs and DeepCopy after API type changes
- `task lint` — lint and auto-format
- `task proto` — regenerate protobuf files
- `task gateway` — start gateway locally (port 8080, playground enabled)
- `task listener` — start listener locally

See [README.md](README.md) for architecture details and configuration reference.

## License

By contributing, you agree that your contributions will be licensed under the [Apache-2.0 License](LICENSE).
