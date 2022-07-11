#!/bin/bash
set -o errexit -o nounset -o pipefail
command -v shellcheck >/dev/null && shellcheck "$0"

echo "Install betterproto..."
pip install --upgrade markupsafe==2.0.1 betterproto[compiler]==2.0.0b4

OUT_DIR="./thornode_proto"

rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}"

echo "Processing thornode proto files..."
rm -rf thornode && git clone https://gitlab.com/thorchain/thornode.git
THOR_DIR="./thornode/proto"
THOR_THIRD_PARTY_DIR="./thornode/third_party/proto"

protoc \
  -I ${THOR_DIR} \
  -I ${THOR_THIRD_PARTY_DIR} \
  --python_betterproto_out="${OUT_DIR}" \
  $(find ${THOR_DIR} -path -prune -o -name '*.proto' -print0 | xargs -0)
