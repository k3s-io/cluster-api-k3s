FROM gcr.io/distroless/static:latest
WORKDIR /
COPY bin/manager ./
USER nobody
ENTRYPOINT ["/manager"]
