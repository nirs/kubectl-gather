# End to end tests

The e2e module provide the *e2e* tool for creating test clusters,
helpers for testing, and tests.

## Requirement

macOS:

```
brew install kind podman
```

## Running the tests

```
make test
```

This creates test clusters if needed and run the tests.

## Cleaning up

```
make clean
```

This delete the test clusters, kubeconfigs, and data gathered during the
tests.
