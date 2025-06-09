# Contributing to openMFP

[Back to the Main Readme](../README.md)

We want to make contributing to this project as easy and transparent as possible.

## Our Development Process
We use GitHub to track issues and feature requests, as well as accept pull requests.

## Pull Requests
You are welcome to contribute with your pull requests. These steps explain the contribution process:

1. Fork the repository and create your branch from `main`.
1. Verify and test your changes see [Testing](#testing).
1. Sign the Developer Certificate of Origin (DCO).

## Testing

```shell
task test
```

### Run a Single Test
For this you need to export the `KUBEBUILDER_ASSETS` environment variable:
```shell
KUBEBUILDER_ASSETS=$(pwd)/bin/k8s/$DIR_WITH_ASSETS
# where $DIR_WITH_ASSETS is the directory that contains binaries for your OS.
o
```

### Live Local Test

See [Local Test](local_test.md).

### See Test Coverage

You can check the coverage as HTML report:
```shell
task coverage-html
```
P.S. If you want to exclude some files from the coverage report, you can add them to the `.testcoverage.yml` file.

### Linting

```shell
task lint
```

## Issues
We use GitHub issues to track bugs. Please ensure your description is
clear and includes sufficient instructions to reproduce the issue.

## License
By contributing to openMFP, you agree that your contributions will be licensed
under its [Apache-2.0 license](../LICENSE).


