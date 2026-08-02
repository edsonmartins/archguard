#!/bin/bash

if [ -z "${driverName:-}" ]; then
  export driverName=sqlite
fi
if [ -z "${dataSourceName:-}" ]; then
  export dataSourceName="file:casdoor.db?cache=shared"
fi

# Custódia OpenBao (perfil conforme, INV-7/ADR-0012): o token do cofre é entregue como
# Docker secret (arquivo), mas o core o lê de VAULT_TOKEN (env — nunca app.conf). Fazemos a
# ponte aqui, sem expor o valor: se VAULT_TOKEN não veio no ambiente e o secret existe,
# exporta a partir do arquivo. Sem isso, o perfil conforme fica fail-closed na custódia.
if [ -z "${VAULT_TOKEN:-}" ] && [ -f /run/secrets/archguard_vault_token ]; then
  export VAULT_TOKEN="$(cat /run/secrets/archguard_vault_token)"
fi

exec /server
