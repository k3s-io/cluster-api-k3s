# e2e test
The e2e test use the [Cluster API test framework](https://pkg.go.dev/sigs.k8s.io/cluster-api/test/framework?tab=doc) and use the [CAPD](https://github.com/kubernetes-sigs/cluster-api/tree/main/test/infrastructure/docker) as the infrastructure provider. Please make sure you have [Docker](https://docs.docker.com/install/) and [kind](https://kind.sigs.k8s.io/) installed.

You could refer to the [Testing Cluster API](https://cluster-api.sigs.k8s.io/developer/testing) for more information.

## Run the e2e test
```shell
make docker-build-e2e   # should be run everytime you change the controller code
make test-e2e   # run all e2e tests
```
To run a specific e2e test, you can use the `GINKGO_FOCUS` environment variable to specify the test you want to run. For example:
```shell
make test-e2e GINKGO_FOCUS="Creating a cluster"
```
Also it is quite useful to run the e2e test with [tilt](https://cluster-api.sigs.k8s.io/developer/tilt), so that you will not need to rebuild docker image with makefile everytime.
## Develop an e2e test
You could refer to [Developing E2E tests](https://cluster-api.sigs.k8s.io/developer/e2e) for a complete guide for developing e2e tests.