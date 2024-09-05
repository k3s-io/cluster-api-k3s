package etcd

import (
	_ "embed"
)

const EtcdProxyDaemonsetYamlLocation = "/var/lib/rancher/k3s/server/manifests/etcd-proxy.yaml"

//go:embed etcd-proxy.yaml
var EtcdProxyDaemonsetYamlTemplate string
