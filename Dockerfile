FROM gcr.io/distroless/static:nonroot
ARG ARCH=arm64
WORKDIR /
COPY bin/${ARCH}/manager ./
USER nonroot:nonroot
ENTRYPOINT ["/manager"]
