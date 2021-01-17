#!/bin/sh 

# see pre-requests:
# - https://grpc.io/docs/languages/go/quickstart/
# - gocosmos plugin is automatically installed during scaffolding.

# set -eo pipefail

# delete existing protobuf generated files
find . -name "*.pb.go" -delete

protoc_gen_gocosmos() {
  go get github.com/regen-network/cosmos-proto/protoc-gen-gocosmos 2>/dev/null
}

protoc_gen_doc() {
  go get -u github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc 2>/dev/null
}

protoc_gen_gocosmos
protoc_gen_doc

proto_dirs=$(find ./proto -path -prune -o -name '*.proto' -print0 | xargs -0 -n1 dirname | sort | uniq)
for dir in $proto_dirs; do
  protoc \
  -I "proto" \
  -I "third_party/proto" \
  --gocosmos_out=plugins=interfacetype+grpc,\
Mgoogle/protobuf/any.proto=github.com/cosmos/cosmos-sdk/codec/types:. \
  $(find "${dir}" -maxdepth 1 -name '*.proto')
done

# move proto files to the right places
cp -r gitlab.com/thorchain/thornode/* ./
rm -rf gitlab.com
