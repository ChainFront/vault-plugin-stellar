#!/usr/bin/env bash
set -e

#
# Helper script for local development. Automatically builds and registers the
# plugin. Requires `vault` is installed and available on $PATH.
#

echo "    Starting"
vault server \
  -dev \
  -dev-root-token-id="root" \
  -log-level="debug" \
  -config="/etc/vault.hcl" \
  &
sleep 2
VAULT_PID=$!

function cleanup {
  echo ""
  echo "==> Cleaning up"
  kill -INT "$VAULT_PID"
}
trap cleanup EXIT

export VAULT_ADDR=http://192.168.50.4:8200

echo "    Authing"
vault auth root &>/dev/null

echo "--> Copying plugin"
cp /vagrant/vault-plugin-stellar /etc/vault.d/plugins
SHASUM=$(sha256sum "/etc/vault.d/plugins/vault-plugin-stellar" | cut -d " " -f1)

echo "    Registering plugin"
vault write sys/plugins/catalog/stellar-plugin \
  sha_256="$SHASUM" \
  command="vault-plugin-stellar"

echo "    Mounting plugin"
vault secrets enable -path=stellar -plugin-name=stellar-plugin plugin

echo "==> Ready!"
wait $!