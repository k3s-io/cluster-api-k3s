# Developing Cluster API K3s with Tilt
This document describes how to use kind and [Tilt][tilt] for a simplified workflow that offers easy deployments and rapid iterative builds. It helps you setup a local development environment with a kind cluster as the management cluster and [CAPD][capd] as the infrastructure provider.

Also, visit the [Cluster API documentation on Tilt][cluster_api_tilt] for more information on how to set up your development environment.

[tilt]: https://tilt.dev
[cluster_api_tilt]: https://cluster-api.sigs.k8s.io/developer/tilt.html
[capd]: https://github.com/kubernetes-sigs/cluster-api/tree/main/test/infrastructure/docker

## Getting started

1. You need to follow the [Prerequisites in Cluster API documentation on Tilt](https://cluster-api.sigs.k8s.io/developer/tilt.html#prerequisites) to have everything installed.

2. Get the source for cluster-api repo
  
    ```bash
    git clone https://github.com/kubernetes-sigs/cluster-api.git
    ```

    In this guide, the `cluster-api` repo is put in the same directory as the `cluster-api-k3s` repo. It is fine if you put it in a different directory, but you might need to change the paths in the `tilt-settings.yaml`.
    
    ```bash
    $ ls
    cluster-api  cluster-api-k3s
    ```

3. Create the management cluster
    
    ```bash
    cd cluster-api
    ./hack/kind-install-for-capd.sh
    ```

4. Create the `tilt-settings.yaml` file and place it in your local copy of `cluster-api`. Here is an example:
    ```yaml
    # refer to https://cluster-api.sigs.k8s.io/developer/tilt.html for documentation
    allowed_contexts:
    - kind-capi-test
    trigger_mode: manual  # set to auto to enable auto-rebuilding
    default_registry: ''  # empty means use local registry 
    provider_repos:
    - ../cluster-api-k3s  # load k3s as a provider, change to a different path if needed
    enable_providers:
    - docker
    - k3s-bootstrap
    - k3s-control-plane
    deploy_observability:
    - visualizer
    kustomize_substitutions:
      # enable some experiment features
      CLUSTER_TOPOLOGY: "true"
      EXP_MACHINE_POOL: "true"
      EXP_CLUSTER_RESOURCE_SET: "true"
      EXP_KUBEADM_BOOTSTRAP_FORMAT_IGNITION: "true"
      EXP_RUNTIME_SDK: "true"
      # add variables for workload cluster template
      KUBERNETES_VERSION: "v1.28.6+k3s2"
      KIND_IMAGE_VERSION: "v1.28.0"
      WORKER_MACHINE_COUNT: "1"
      CONTROL_PLANE_MACHINE_COUNT: "1"
      # Note: kustomize substitutions expects the values to be strings. This can be achieved by wrapping the values in quotation marks.
      # also, can use this to provide credentials
    kind_cluster_name: capi-test
    extra_args:
      # add extra arguments when launching the binary 
      k3s-bootstrap:
      - --enable-leader-election=false
      k3s-control-plane:
      - --enable-leader-election=false
    debug: 
      # enable delve for debugging
      docker:
        continue: true  # change to false if you need the service to be running after the delve has been connected
        port: 30000
        profiler_port: 30001
        metrics_port: 30002
      core:
        continue: true
        port: 31000
        profiler_port: 31001
        metrics_port: 31002
      k3s-bootstrap:
        continue: true
        port: 32000
      k3s-control-plane:
        continue: true
        port: 33000
    template_dirs:
      # add template for fast workload cluster creation, change to a different path if needed
      # you could also add more templates
      k3s-bootstrap:
      # please run `make generate-e2e-templates` to generate the templates first
      - ../cluster-api-k3s/test/e2e/data/infrastructure-docker
    ```
5. Run `tilt` in the `cluster-api` directory

    ```bash
    tilt up
    ```

6. Create a cluster using the `tilt` UI: if you have `template_dirs` set in the `tilt-settings.yaml`, you will find the `CABP3.templates` resource and you could create a cluster with one click!

## Debugging

If you would like to debug CAPI K3s (or core CAPI / another provider) you can run the provider with delve. This will then allow you to attach to delve and debug. Please check the `debug` part in `tilt-settings.yaml`.

For vscode, you can use a launch configuration like this (please change the `port` to the one you have set in `tilt-settings.yaml`):
```json
{
    "name": "K3s Bootstrap Controller",
    "type": "go",
    "request": "attach",
    "mode": "remote",
    "remotePath": "",
    "port": 32000,
    "host": "127.0.0.1",
    "showLog": true,
    "trace": "log",
    "logOutput": "rpc"
}
```
## Clean up
Before deleting the kind cluster, make sure you delete all the workload clusters.
```bash
kubectl delete cluster <clustername>
tilt up (ctrl-c)
kind delete cluster
```