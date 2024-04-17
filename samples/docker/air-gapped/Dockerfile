FROM kindest/node:v1.28.0
ARG K3S_VERSION=v1.28.6+k3s2

# Load docker images
# Note that the flow follows the manually deploy image steps, but private registry method should also be supported
RUN mkdir -p /var/lib/rancher/k3s/agent/images/
RUN curl -L -o /var/lib/rancher/k3s/agent/images/k3s-airgap-images-amd64.tar.zst "https://github.com/k3s-io/k3s/releases/download/${K3S_VERSION}/k3s-airgap-images-amd64.tar.zst"

# Download install script to /opt/install.sh
RUN curl -L -o /opt/install.sh  https://get.k3s.io
RUN chmod +x /opt/install.sh

# Download k3s binary
RUN curl -L -o /usr/local/bin/k3s "https://github.com/k3s-io/k3s/releases/download/${K3S_VERSION}/k3s"
RUN chmod +x /usr/local/bin/k3s
