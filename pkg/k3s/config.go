package k3s

import (
	"fmt"
	"strings"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
)

const DefaultK3sConfigLocation = "/etc/rancher/k3s/config.yaml"

type K3sServerConfig struct {
	DisableCloudController    bool     `json:"disable-cloud-controller,omitempty"`
	KubeAPIServerArgs         []string `json:"kube-apiserver-arg,omitempty"`
	KubeControllerManagerArgs []string `json:"kube-controller-manager-arg,omitempty"`
	KubeSchedulerArgs         []string `json:"kube-scheduler-arg,omitempty"`
	TLSSan                    []string `json:"tls-san,omitempty"`
	BindAddress               string   `json:"bind-address,omitempty"`
	HTTPSListenPort           string   `json:"https-listen-port,omitempty"`
	AdvertiseAddress          string   `json:"advertise-address,omitempty"`
	AdvertisePort             string   `json:"advertise-port,omitempty"`
	ClusterCidr               string   `json:"cluster-cidr,omitempty"`
	ServiceCidr               string   `json:"service-cidr,omitempty"`
	ClusterDNS                string   `json:"cluster-dns,omitempty"`
	ClusterDomain             string   `json:"cluster-domain,omitempty"`
	DisableComponents         []string `json:"disable,omitempty"`
	ClusterInit               bool     `json:"cluster-init,omitempty"`
	SystemDefaultRegistry     string   `json:"system-default-registry,omitempty"`
	K3sAgentConfig            `json:",inline"`
}

type K3sAgentConfig struct {
	Token           string   `json:"token,omitempty"`
	Server          string   `json:"server,omitempty"`
	KubeletArgs     []string `json:"kubelet-arg,omitempty"`
	NodeLabels      []string `json:"node-label,omitempty"`
	NodeTaints      []string `json:"node-taint,omitempty"`
	PrivateRegistry string   `json:"private-registry,omitempty"`
	KubeProxyArgs   []string `json:"kube-proxy-arg,omitempty"`
	NodeName        string   `json:"node-name,omitempty"`
}

func GenerateInitControlPlaneConfig(controlPlaneEndpoint string, token string, serverConfig bootstrapv1.KThreesServerConfig, agentConfig bootstrapv1.KThreesAgentConfig) K3sServerConfig {
	kubeletExtraArgs := getKubeletExtraArgs(serverConfig)
	k3sServerConfig := K3sServerConfig{
		DisableCloudController:    getDisableCloudController(serverConfig),
		ClusterInit:               true,
		KubeAPIServerArgs:         append(serverConfig.KubeAPIServerArgs, "anonymous-auth=true", getTLSCipherSuiteArg()),
		TLSSan:                    append(serverConfig.TLSSan, controlPlaneEndpoint),
		KubeControllerManagerArgs: append(serverConfig.KubeControllerManagerArgs, kubeletExtraArgs...),
		KubeSchedulerArgs:         serverConfig.KubeSchedulerArgs,
		BindAddress:               serverConfig.BindAddress,
		HTTPSListenPort:           serverConfig.HTTPSListenPort,
		AdvertiseAddress:          serverConfig.AdvertiseAddress,
		AdvertisePort:             serverConfig.AdvertisePort,
		ClusterCidr:               serverConfig.ClusterCidr,
		ServiceCidr:               serverConfig.ServiceCidr,
		ClusterDNS:                serverConfig.ClusterDNS,
		ClusterDomain:             serverConfig.ClusterDomain,
		DisableComponents:         serverConfig.DisableComponents,
		SystemDefaultRegistry:     serverConfig.SystemDefaultRegistry,
	}

	k3sServerConfig.K3sAgentConfig = K3sAgentConfig{
		Token:           token,
		KubeletArgs:     append(agentConfig.KubeletArgs, kubeletExtraArgs...),
		NodeLabels:      agentConfig.NodeLabels,
		NodeTaints:      agentConfig.NodeTaints,
		PrivateRegistry: agentConfig.PrivateRegistry,
		KubeProxyArgs:   agentConfig.KubeProxyArgs,
		NodeName:        agentConfig.NodeName,
	}

	return k3sServerConfig
}

func GenerateJoinControlPlaneConfig(serverURL string, token string, controlplaneendpoint string, serverConfig bootstrapv1.KThreesServerConfig, agentConfig bootstrapv1.KThreesAgentConfig) K3sServerConfig {
	kubeletExtraArgs := getKubeletExtraArgs(serverConfig)
	k3sServerConfig := K3sServerConfig{
		DisableCloudController:    getDisableCloudController(serverConfig),
		KubeAPIServerArgs:         append(serverConfig.KubeAPIServerArgs, "anonymous-auth=true", getTLSCipherSuiteArg()),
		TLSSan:                    append(serverConfig.TLSSan, controlplaneendpoint),
		KubeControllerManagerArgs: append(serverConfig.KubeControllerManagerArgs, kubeletExtraArgs...),
		KubeSchedulerArgs:         serverConfig.KubeSchedulerArgs,
		BindAddress:               serverConfig.BindAddress,
		HTTPSListenPort:           serverConfig.HTTPSListenPort,
		AdvertiseAddress:          serverConfig.AdvertiseAddress,
		AdvertisePort:             serverConfig.AdvertisePort,
		ClusterCidr:               serverConfig.ClusterCidr,
		ServiceCidr:               serverConfig.ServiceCidr,
		ClusterDNS:                serverConfig.ClusterDNS,
		ClusterDomain:             serverConfig.ClusterDomain,
		DisableComponents:         serverConfig.DisableComponents,
		SystemDefaultRegistry:     serverConfig.SystemDefaultRegistry,
	}

	k3sServerConfig.K3sAgentConfig = K3sAgentConfig{
		Token:           token,
		Server:          serverURL,
		KubeletArgs:     append(agentConfig.KubeletArgs, kubeletExtraArgs...),
		NodeLabels:      agentConfig.NodeLabels,
		NodeTaints:      agentConfig.NodeTaints,
		PrivateRegistry: agentConfig.PrivateRegistry,
		KubeProxyArgs:   agentConfig.KubeProxyArgs,
		NodeName:        agentConfig.NodeName,
	}

	return k3sServerConfig
}

func GenerateWorkerConfig(serverURL string, token string, serverConfig bootstrapv1.KThreesServerConfig, agentConfig bootstrapv1.KThreesAgentConfig) K3sAgentConfig {
	kubeletExtraArgs := getKubeletExtraArgs(serverConfig)
	return K3sAgentConfig{
		Server:          serverURL,
		Token:           token,
		KubeletArgs:     append(agentConfig.KubeletArgs, kubeletExtraArgs...),
		NodeLabels:      agentConfig.NodeLabels,
		NodeTaints:      agentConfig.NodeTaints,
		PrivateRegistry: agentConfig.PrivateRegistry,
		KubeProxyArgs:   agentConfig.KubeProxyArgs,
		NodeName:        agentConfig.NodeName,
	}
}

func getTLSCipherSuiteArg() string {
	/**
	Can't use this method because k3s is using older apiserver pkgs that hardcode a subset of ciphers.
	https://github.com/k3s-io/k3s/blob/master/vendor/k8s.io/component-base/cli/flag/ciphersuites_flag.go#L29
	csList := ""
	css := tls.CipherSuites()
	for _, cs := range css {
		csList += cs.Name + ","
	}
	csList = strings.TrimRight(csList, ",")
	**/

	ciphers := []string{
		// Modern Compatibility recommended configuration in
		// https://wiki.mozilla.org/Security/Server_Side_TLS#Modern_compatibility
		"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
		"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
		"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",

		// Few Intermediate Compatibility ones to support AWS ELB, that don't support forward secrecy.
		// note: golang tls library doesn't support all intermediate cipher suites.
		"TLS_RSA_WITH_AES_128_GCM_SHA256",
		"TLS_RSA_WITH_AES_256_GCM_SHA384",
	}

	ciphersList := ""
	for _, cc := range ciphers {
		ciphersList += cc + ","
	}
	ciphersList = strings.TrimRight(ciphersList, ",")

	return fmt.Sprintf("tls-cipher-suites=%s", ciphersList)
}

func getKubeletExtraArgs(serverConfig bootstrapv1.KThreesServerConfig) []string {
	kubeletExtraArgs := []string{}
	if serverConfig.CloudProviderName != nil && len(*serverConfig.CloudProviderName) > 0 {
		cloudProviderArg := fmt.Sprintf("cloud-provider=%s", *serverConfig.CloudProviderName)
		kubeletExtraArgs = append(kubeletExtraArgs, cloudProviderArg)
	}
	return kubeletExtraArgs
}

func getDisableCloudController(serverConfig bootstrapv1.KThreesServerConfig) bool {
	if serverConfig.DisableCloudController == nil {
		return true
	}
	return *serverConfig.DisableCloudController
}
