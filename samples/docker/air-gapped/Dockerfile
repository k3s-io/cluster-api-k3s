FROM kindest/node:v1.28.0@sha256:b7a4cad12c197af3ba43202d3efe03246b3f0793f162afb40a33c923952d5b31
ARG K3S_VERSION=v1.28.6+k3s2
ARG K3S_VERSION_COMMIT=c9f49a3b06cd7ebe793f8cc1dcd0293168e743d9
ARG K3S_BIN_SUM=eb1274cc1de8f8cec361b94470c4c781f93eafa657ee96cf318f9d3e28b58857
ARG K3S_IMAGE_AMD64_TAR_ZST_SUM=ae5af772bb5b037850448afb64c89f9e8bfca3017cc14095ba7ded00ae67bfab
ARG INSTALL_SCRIPT_PATH=/opt/install.sh
ARG ALPINE_SOCAT_TARBALL_PATH=./alpine-socat.tar
ARG K3S_INSTALL_SCRIPT_SUM=5f785120f00ef4b0aba205161232d2a04b6e7a75332cae7059fcc1f517340777

# Load docker images
# Note that the flow follows the manually deploy image steps, but private registry method should also be supported
RUN mkdir -p /var/lib/rancher/k3s/agent/images/
RUN curl -L -o /var/lib/rancher/k3s/agent/images/k3s-airgap-images-amd64.tar.zst "https://github.com/k3s-io/k3s/releases/download/${K3S_VERSION}/k3s-airgap-images-amd64.tar.zst" && \
    echo "${K3S_IMAGE_AMD64_TAR_ZST_SUM}  /var/lib/rancher/k3s/agent/images/k3s-airgap-images-amd64.tar.zst" | sha256sum -c -
# Copy alpine-socat.tar to /var/lib/rancher/k3s/agent/images/
COPY $ALPINE_SOCAT_TARBALL_PATH /var/lib/rancher/k3s/agent/images/

# Download install script to $INSTALL_SCRIPT_PATH
RUN install -d "$(dirname "${INSTALL_SCRIPT_PATH}")"
RUN curl -L -o $INSTALL_SCRIPT_PATH "https://raw.githubusercontent.com/k3s-io/k3s/${K3S_VERSION_COMMIT}/install.sh" && \
    echo "${K3S_INSTALL_SCRIPT_SUM}  $INSTALL_SCRIPT_PATH" | sha256sum -c
RUN chmod +x $INSTALL_SCRIPT_PATH

# Download k3s binary
RUN curl -L -o /usr/local/bin/k3s "https://github.com/k3s-io/k3s/releases/download/${K3S_VERSION}/k3s" && \
    echo "${K3S_BIN_SUM}  /usr/local/bin/k3s" | sha256sum -c -
RUN chmod +x /usr/local/bin/k3s
