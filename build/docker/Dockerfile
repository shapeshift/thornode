#
# THORChain
#

#
# Build
#
FROM golang:1.15.5 AS build

ENV GOBIN=/go/bin
ENV GOPATH=/go
ENV CGO_ENABLED=0
ENV GOOS=linux

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TAG=mainnet
RUN make install

#
# Main
#
FROM alpine

RUN apk add --update gcc musl-dev python3-dev libffi-dev openssl-dev jq curl bind-tools py3-pip && \
    rm -rf /var/cache/apk/*

RUN pip3 install requests==2.22.0 web3==5.12.0

# Copy the compiled binaires over.
COPY --from=build /go/bin/generate /usr/bin/
COPY --from=build /go/bin/thord /usr/bin/
COPY --from=build /go/bin/thorcli /usr/bin/
COPY --from=build /go/bin/bifrost /usr/bin/

COPY build/scripts /scripts
