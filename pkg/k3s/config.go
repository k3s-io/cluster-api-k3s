package k3s

import (
	bootstrapv1 "github.com/zawachte-msft/cluster-api-k3s/bootstrap/api/v1alpha3"
)

const DefaultK3sConfigLocation = "/etc/rancher/k3s/config.yaml"

type K3sServerConfig struct {
	DisableCloudController    bool     `json:"disable-cloud-controller,omitempty"`
	KubeAPIServerArgs         []string `json:"kube-apiserver-arg,omitempty"`
	KubeControllerManagerArgs []string `json:"kube-controller-manager-arg,omitempty"`
	TLSSan                    []string `json:"tls-san,omitempty"`
	BindAddress               string   `json:"bind-address,omitempty"`
	HttpsListenPort           string   `json:"https-listen-port,omitempty"`
	AdvertiseAddress          string   `json:"advertise-address,omitempty"`
	AdvertisePort             string   `json:"advertise-port,omitempty"`
	ClusterCidr               string   `json:"cluster-cidr,omitempty"`
	ServiceCidr               string   `json:"service-cidr,omitempty"`
	ClusterDNS                string   `json:"cluster-dns,omitempty"`
	ClusterDomain             string   `json:"cluster-domain,omitempty"`
	DisableComponents         []string `json:"disable,omitempty"`
	ClusterInit               bool     `json:"cluster-init,omitempty"`
	K3sAgentConfig            `json:",inline"`
}

type K3sAgentConfig struct {
	Token           string   `json:"token,omitempty"`
	Server          string   `json:"server,omitempty"`
	KubeletArgs     []string `json:"kubelet-arg,omitempty"`
	NodeLabels      []string `json:"node-labels,omitempty"`
	NodeTaints      []string `json:"node-taints,omitempty"`
	PrivateRegistry string   `json:"private-registry,omitempty"`
	KubeProxyArgs   []string `json:"kube-proxy-arg,omitempty"`
}

func GenerateInitControlPlaneConfig(controlPlaneEndpoint string, token string, serverConfig bootstrapv1.KThreesServerConfig, agentConfig bootstrapv1.KThreesAgentConfig) K3sServerConfig {
	k3sServerConfig := K3sServerConfig{
		DisableCloudController:    true,
		ClusterInit:               true,
		KubeAPIServerArgs:         append(serverConfig.KubeAPIServerArgs, "anonymous-auth=true"),
		TLSSan:                    append(serverConfig.TLSSan, controlPlaneEndpoint),
		KubeControllerManagerArgs: append(serverConfig.KubeControllerManagerArgs, "cloud-provider=external"),
		BindAddress:               serverConfig.BindAddress,
		HttpsListenPort:           serverConfig.HttpsListenPort,
		AdvertiseAddress:          serverConfig.AdvertiseAddress,
		AdvertisePort:             serverConfig.AdvertisePort,
		ClusterCidr:               serverConfig.ClusterCidr,
		ServiceCidr:               serverConfig.ServiceCidr,
		ClusterDNS:                serverConfig.ClusterDNS,
		ClusterDomain:             serverConfig.ClusterDomain,
		DisableComponents:         serverConfig.DisableComponents,
	}

	k3sServerConfig.K3sAgentConfig = K3sAgentConfig{
		Token:           token,
		KubeletArgs:     append(agentConfig.KubeletArgs, "cloud-provider=external"),
		NodeLabels:      agentConfig.NodeLabels,
		NodeTaints:      agentConfig.NodeTaints,
		PrivateRegistry: agentConfig.PrivateRegistry,
		KubeProxyArgs:   agentConfig.KubeProxyArgs,
	}

	return k3sServerConfig
}

func GenerateJoinControlPlaneConfig(serverUrl string, token string, serverConfig bootstrapv1.KThreesServerConfig, agentConfig bootstrapv1.KThreesAgentConfig) K3sServerConfig {
	k3sServerConfig := K3sServerConfig{
		DisableCloudController:    true,
		KubeAPIServerArgs:         append(serverConfig.KubeAPIServerArgs, "anonymous-auth=true"),
		TLSSan:                    serverConfig.TLSSan,
		KubeControllerManagerArgs: append(serverConfig.KubeControllerManagerArgs, "cloud-provider=external"),
		BindAddress:               serverConfig.BindAddress,
		HttpsListenPort:           serverConfig.HttpsListenPort,
		AdvertiseAddress:          serverConfig.AdvertiseAddress,
		AdvertisePort:             serverConfig.AdvertisePort,
		ClusterCidr:               serverConfig.ClusterCidr,
		ServiceCidr:               serverConfig.ServiceCidr,
		ClusterDNS:                serverConfig.ClusterDNS,
		ClusterDomain:             serverConfig.ClusterDomain,
		DisableComponents:         serverConfig.DisableComponents,
	}

	k3sServerConfig.K3sAgentConfig = K3sAgentConfig{
		Token:           token,
		Server:          serverUrl,
		KubeletArgs:     append(agentConfig.KubeletArgs, "cloud-provider=external"),
		NodeLabels:      agentConfig.NodeLabels,
		NodeTaints:      agentConfig.NodeTaints,
		PrivateRegistry: agentConfig.PrivateRegistry,
		KubeProxyArgs:   agentConfig.KubeProxyArgs,
	}

	return k3sServerConfig
}

func GenerateWorkerConfig(serverUrl string, token string, agentConfig bootstrapv1.KThreesAgentConfig) K3sAgentConfig {
	return K3sAgentConfig{
		Server:          serverUrl,
		Token:           token,
		KubeletArgs:     append(agentConfig.KubeletArgs, "cloud-provider=external"),
		NodeLabels:      agentConfig.NodeLabels,
		NodeTaints:      agentConfig.NodeTaints,
		PrivateRegistry: agentConfig.PrivateRegistry,
		KubeProxyArgs:   agentConfig.KubeProxyArgs,
	}
}
