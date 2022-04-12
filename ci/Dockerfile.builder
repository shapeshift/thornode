FROM golang:1.17.0


# hadolint ignore=DL3008,DL4006
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
    curl git jq make protobuf-compiler xz-utils sudo \
    && rm -rf /var/cache/apt/lists \
    && go install mvdan.cc/gofumpt@v0.3.0 \
    && curl https://get.trunk.io -fsSL | bash -s -- -y
