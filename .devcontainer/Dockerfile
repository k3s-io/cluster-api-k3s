FROM jetpackio/devbox:0.17.0@sha256:00d28f21397c15fdf451beb8a84dea5b726f77f08ff166f08fd2d3928605aac5

# Installing your devbox project
WORKDIR /code
USER root:root
RUN mkdir -p /code && chown ${DEVBOX_USER}:${DEVBOX_USER} /code
USER ${DEVBOX_USER}:${DEVBOX_USER}
COPY --chown=${DEVBOX_USER}:${DEVBOX_USER} devbox.json devbox.json
COPY --chown=${DEVBOX_USER}:${DEVBOX_USER} devbox.lock devbox.lock



RUN devbox run -- echo "Installed Packages." && nix-store --gc && nix-store --optimise

RUN devbox shellenv --init-hook >> ~/.profile
