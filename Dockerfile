FROM gcr.io/distroless/static:nonroot
ARG ARCH=amd64
WORKDIR /
COPY bin/${ARCH}/manager ./
USER nonroot:nonroot
ENTRYPOINT ["/manager"]
