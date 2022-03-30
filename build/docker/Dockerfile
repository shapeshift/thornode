#
# THORChain
#

#
# Build
#
FROM golang:1.17.0 AS build

ENV GOBIN=/go/bin
ENV GOPATH=/go
ENV CGO_ENABLED=0
ENV GOOS=linux

RUN apt-get update -q && apt-get install -y protobuf-compiler

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TAG=mainnet
RUN make install

#
# Main
#
FROM alpine:latest

RUN apk add --no-cache gcc musl-dev python3-dev libffi-dev openssl-dev jq curl bind-tools py3-pip protoc

RUN pip3 install --no-cache-dir requests==2.22.0 web3==5.12.0

# Copy the compiled binaires over.
COPY --from=build /go/bin/generate /usr/bin/
COPY --from=build /go/bin/thornode /usr/bin/
COPY --from=build /go/bin/bifrost /usr/bin/

COPY build/scripts /scripts

# default to mainnet
ARG TAG=mainnet
ENV NET=$TAG

# default to fullnode
CMD ["/scripts/fullnode.sh"]
