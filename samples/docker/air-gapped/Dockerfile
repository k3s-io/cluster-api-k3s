FROM kindest/node:v1.28.0
ARG K3S_VERSION=v1.28.6+k3s2
ARG INSTALL_SCRIPT_PATH=/opt/install.sh
ARG ALPINE_SOCAT_TARBALL_PATH=./alpine-socat.tar

# Load docker images
# Note that the flow follows the manually deploy image steps, but private registry method should also be supported
RUN mkdir -p /var/lib/rancher/k3s/agent/images/
RUN curl -L -o /var/lib/rancher/k3s/agent/images/k3s-airgap-images-amd64.tar.zst "https://github.com/k3s-io/k3s/releases/download/${K3S_VERSION}/k3s-airgap-images-amd64.tar.zst"
# Copy alpine-socat.tar to /var/lib/rancher/k3s/agent/images/
COPY $ALPINE_SOCAT_TARBALL_PATH /var/lib/rancher/k3s/agent/images/

# Download install script to $INSTALL_SCRIPT_PATH
RUN curl -L -o $INSTALL_SCRIPT_PATH  https://get.k3s.io
RUN chmod +x $INSTALL_SCRIPT_PATH

# Download k3s binary
RUN curl -L -o /usr/local/bin/k3s "https://github.com/k3s-io/k3s/releases/download/${K3S_VERSION}/k3s"
RUN chmod +x /usr/local/bin/k3s
