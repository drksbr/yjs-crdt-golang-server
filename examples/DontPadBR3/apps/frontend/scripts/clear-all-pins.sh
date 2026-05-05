#!/bin/sh
# ─────────────────────────────────────────────────────────────────────────────
# clear-all-pins.sh
#
# Remove o PIN de todos os documentos do DontPad (produção via Docker).
#
# Uso:
#   ./scripts/clear-all-pins.sh
#
# Pré-requisito: docker compose up (containers rodando)
# ─────────────────────────────────────────────────────────────────────────────

set -e

# Nome do container do app Next.js (ajuste se o seu projeto tem nome diferente)
APP_CONTAINER="${APP_CONTAINER:-$(docker compose ps -q app 2>/dev/null | head -1)}"

if [ -z "$APP_CONTAINER" ]; then
  echo "❌ Container do app não encontrado. Verifique se 'docker compose up' está rodando."
  exit 1
fi

echo "🔍 Descobrindo documentos no container ysweet..."

# Lista os IDs de todos os docs no volume do Y-Sweet
YSWEET_CONTAINER="${YSWEET_CONTAINER:-$(docker compose ps -q ysweet 2>/dev/null | head -1)}"

if [ -z "$YSWEET_CONTAINER" ]; then
  echo "❌ Container ysweet não encontrado."
  exit 1
fi

# Pega os IDs dos docs do volume ysweet (/data tem subpastas por doc ID)
DOC_IDS=$(docker exec "$YSWEET_CONTAINER" sh -c 'ls /data 2>/dev/null' | tr '\n' ',' | sed 's/,$//')

if [ -z "$DOC_IDS" ]; then
  echo "⚠️  Nenhum documento encontrado em /data no container ysweet."
  echo "   Os documentos podem não existir ainda ou o caminho pode ser diferente."
  exit 0
fi

echo "📋 Documentos encontrados: $(echo "$DOC_IDS" | tr ',' '\n' | wc -l | tr -d ' ')"
echo ""

# Executa o script Node.js dentro do container do app
# O container do app tem acesso HTTP ao Y-Sweet via rede interna (dontpad-network)
docker exec \
  -e "DOC_IDS=$DOC_IDS" \
  -e "CONNECTION_STRING=${CONNECTION_STRING:-ys://ysweet:8080}" \
  -e "YSWEET_AUTH_TOKEN=${YSWEET_AUTH_TOKEN:-}" \
  "$APP_CONTAINER" \
  node /app/scripts/clear-pins.mjs
