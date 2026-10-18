package k3s

import (
	"testing"

	. "github.com/onsi/gomega"

	bootstrapv1 "github.com/k3s-io/cluster-api-k3s/bootstrap/api/v1beta2"
)

func TestKubeAPIServerArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "adds the defaults",
			args: []string{"audit-log-maxage=30"},
			want: []string{"audit-log-maxage=30", "anonymous-auth=true", getTLSCipherSuiteArg()},
		},
		{
			name: "keeps the user's values",
			args: []string{"anonymous-auth=false", "--tls-cipher-suites=TLS_AES_128_GCM_SHA256"},
			want: []string{"anonymous-auth=false", "--tls-cipher-suites=TLS_AES_128_GCM_SHA256"},
		},
		{
			name: "leaves out anonymous-auth with an authentication config",
			args: []string{"authentication-config=/etc/rancher/k3s/authentication-config.yaml"},
			want: []string{"authentication-config=/etc/rancher/k3s/authentication-config.yaml", getTLSCipherSuiteArg()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			serverConfig := bootstrapv1.KThreesServerConfig{KubeAPIServerArgs: tt.args}

			initConfig := GenerateInitControlPlaneConfig("10.0.0.1", "token", serverConfig, bootstrapv1.KThreesAgentConfig{})
			g.Expect(initConfig.KubeAPIServerArgs).To(Equal(tt.want))

			joinConfig := GenerateJoinControlPlaneConfig("https://10.0.0.1:6443", "token", "10.0.0.1", serverConfig, bootstrapv1.KThreesAgentConfig{})
			g.Expect(joinConfig.KubeAPIServerArgs).To(Equal(tt.want))
		})
	}
}
