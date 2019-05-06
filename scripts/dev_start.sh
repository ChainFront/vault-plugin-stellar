#!/usr/bin/env bash
set -e

#
# Helper script for local development. Automatically builds and registers the
# plugin. Requires `vault` is installed and available on $PATH.
#

# Get the right dir
DIR="$(cd "$(dirname "$(readlink "$0")")" && pwd)"

echo "==> Starting dev"

echo "--> Scratch dir"
echo "    Creating"
SCRATCH="$DIR/tmp"
mkdir -p "$SCRATCH/plugins"

echo "--> Vault server"
echo "    Writing config"
tee "$SCRATCH/vault.hcl" > /dev/null <<EOF
plugin_directory = "$SCRATCH/plugins"
EOF


echo "    Starting"
vault server \
  -dev \
  -dev-root-token-id="root" \
  -log-level="debug" \
  -config="$SCRATCH/vault.hcl" \
  &
sleep 2
VAULT_PID=$!


function cleanup {
  echo ""
  echo "==> Cleaning up"
  kill -INT "$VAULT_PID"
  rm -rf "$SCRATCH"
}
trap cleanup EXIT


echo "    Authenticating"
export VAULT_ADDR=http://localhost:8200
vault auth root &>/dev/null

echo "--> Creating policies"
vault write sys/policy/user rules=-<<EOF
path "secret/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
EOF
vault write sys/policy/group rules=-<<EOF
path "secret/*" {
  capabilities = ["read"]
}
EOF
vault write sys/policy/usergroup rules=-<<EOF
path "*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
EOF

echo "--> Building"
go build -o "$SCRATCH/plugins/vault-plugin-stellar"

echo "    Registering plugin"
SHASUM=$(shasum -a 256 "$SCRATCH/plugins/vault-plugin-stellar" | cut -d " " -f1)
vault write sys/plugins/catalog/stellar-plugin \
  sha_256="$SHASUM" \
  command="vault-plugin-stellar"

echo "    Mounting plugin"
vault secrets enable -path=stellar -plugin-name=stellar-plugin plugin

echo "==> Ready!"
wait $!
