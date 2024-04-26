# e2e test
The e2e test use the [Cluster API test framework](https://pkg.go.dev/sigs.k8s.io/cluster-api/test/framework?tab=doc) and use the [CAPD](https://github.com/kubernetes-sigs/cluster-api/tree/main/test/infrastructure/docker) as the infrastructure provider. Please make sure you have [Docker](https://docs.docker.com/install/) and [kind](https://kind.sigs.k8s.io/) installed.

You could refer to the [Testing Cluster API](https://cluster-api.sigs.k8s.io/developer/testing) for more information.

## Run the e2e test
The e2e image will be built with tag `dev`. You should build the image first before running the test.
```shell
make docker-build-e2e   # should be run everytime you change the controller code
make test-e2e   # run all e2e tests
```
### Run a specific e2e test
To run a specific e2e test, such as `[PR-Blocking]`, use the `GINKGO_FOCUS` environment variable as shown below:
```shell
make GINKGO_FOCUS="\\[PR-Blocking\\]" test-e2e  # only run e2e test with `[PR-Blocking]` in its spec name
```
### Run the e2e test with tilt
It is quite useful to run the e2e test with [tilt](https://cluster-api.sigs.k8s.io/developer/tilt), so that you will not need to rebuild docker image with `make docker-build-e2e` everytime. Also you will not need to wait a new cluster creation and setup. If you have set up your tilt cluster and made the current context points to this cluster, you could run:
```shell
# running e2e for the cluster pointed by the current context
make USE_EXISTING_CLUSTER=true test-e2e
```
## Develop an e2e test
You could refer to [Developing E2E tests](https://cluster-api.sigs.k8s.io/developer/e2e) for a complete guide for developing e2e tests.

A guide for developing a k3s e2e test:

* Group test specs by scenarios (e.g., `create_test`, `node_scale_test`, `upgrade_test`). Create a new file under `test/e2e/` for new scenarios.
* If a different docker cluster template is needed, create one under `test/e2e/infrastructure-docker/` and link it in `test/e2e/config/k3s-docker.yaml`.
* Define tunable variables in the cluster template as environment variables under `variables` in `test/e2e/config/k3s-docker.yaml`. Enable necessary feature flags here as well (e.g., `EXP_CLUSTER_RESOURCE_SET: "true"`).
* If reusing a [cluster-api test spec](https://github.com/kubernetes-sigs/cluster-api/tree/main/test/e2e), note that they assume the use of `KubeadmControlPlane`. For customization, copy code into `test/e2e/helpers.go`.

## Troubleshooting
* [Cluster API with Docker - "too many open files".](https://cluster-api.sigs.k8s.io/user/troubleshooting.html?highlight=too%20many#cluster-api-with-docker----too-many-open-files)
