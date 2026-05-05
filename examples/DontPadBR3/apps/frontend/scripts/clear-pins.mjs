/**
 * Script de admin: remove o PIN de todos os documentos do Y-Sweet.
 *
 * Uso (local, com y-sweet rodando em localhost:8080):
 *   CONNECTION_STRING=ys://localhost:8080 node scripts/clear-pins.mjs
 *
 * Uso via docker (produção):
 *   docker exec -it dontpadbr2-app-1 node /app/scripts/clear-pins.mjs
 *
 * Variáveis de ambiente:
 *   CONNECTION_STRING  — connection string do Y-Sweet (obrigatório em prod)
 *   YSWEET_AUTH_TOKEN  — token de auth alternativo (opcional)
 *   DOC_IDS            — lista separada por vírgula de doc IDs específicos (opcional)
 *                         Se omitido, lê os IDs do diretório de dados (DATA_DIR)
 *   DATA_DIR           — caminho do diretório de dados local (padrão: ./data)
 *                         Usado apenas para descobrir os IDs dos documentos
 */

import * as Y from "yjs";
import fs from "fs";
import path from "path";

// ─── Helpers de conexão (espelha lib/ysweet.ts) ───────────────────────────

function ysToHttpUrl(conn) {
  return conn
    .replace(/^yss?:\/\/[^@]*@/, "http://")
    .replace(/^yss:\/\//, "https://")
    .replace(/^ys:\/\//, "http://");
}

function extractAuthToken(conn) {
  const match = conn.match(/^ys[s]?:\/\/([^@]+)@/);
  return match
    ? decodeURIComponent(match[1])
    : process.env.YSWEET_AUTH_TOKEN || undefined;
}

const CONNECTION_STRING =
  process.env.CONNECTION_STRING || "ys://localhost:8080";
const HTTP_BASE = ysToHttpUrl(CONNECTION_STRING);
const AUTH_TOKEN = extractAuthToken(CONNECTION_STRING);
const BASES = [HTTP_BASE, "http://localhost:8080", "http://ysweet:8080"];

const authHeaders = {
  "Content-Type": "application/json",
  ...(AUTH_TOKEN ? { Authorization: `Bearer ${AUTH_TOKEN}` } : {}),
};

// ─── Buscar update de um doc ───────────────────────────────────────────────

async function getDocUpdate(docId) {
  for (const base of BASES) {
    try {
      const authRes = await fetch(`${base}/doc/${docId}/auth`, {
        method: "POST",
        headers: authHeaders,
        body: JSON.stringify({ authorization: "full" }),
      });
      if (!authRes.ok) continue;
      const { baseUrl } = await authRes.json();
      if (!baseUrl) continue;

      const docPath = new URL(baseUrl).pathname;
      const updateRes = await fetch(`${base}${docPath}/as-update`, {
        headers: {
          ...(AUTH_TOKEN ? { Authorization: `Bearer ${AUTH_TOKEN}` } : {}),
        },
      });
      if (!updateRes.ok) continue;
      return {
        update: new Uint8Array(await updateRes.arrayBuffer()),
        base,
        docPath,
      };
    } catch {
      // tenta próxima base
    }
  }
  return null;
}

// ─── Postar update de volta ────────────────────────────────────────────────

async function postDocUpdate(base, docPath, updateBytes) {
  // Endpoint: POST {baseUrl}/update com body binário
  const res = await fetch(`${base}${docPath}/update`, {
    method: "POST",
    headers: {
      "Content-Type": "application/octet-stream",
      ...(AUTH_TOKEN ? { Authorization: `Bearer ${AUTH_TOKEN}` } : {}),
    },
    body: updateBytes,
  });
  return res.ok;
}

// ─── Descobrir doc IDs ──────────────────────────────────────────────────────

function discoverDocIds() {
  // 1. Variável de ambiente explícita
  if (process.env.DOC_IDS) {
    return process.env.DOC_IDS.split(",")
      .map((s) => s.trim())
      .filter(Boolean);
  }

  // 2. Diretório local de dados (app-data: contém apenas docs que tiveram upload)
  const DATA_DIR = process.env.DATA_DIR || path.join(process.cwd(), "data");
  if (fs.existsSync(DATA_DIR)) {
    const entries = fs.readdirSync(DATA_DIR, { withFileTypes: true });
    const ids = entries.filter((e) => e.isDirectory()).map((e) => e.name);
    if (ids.length > 0) {
      console.log(`📂 Encontrados ${ids.length} doc(s) em ${DATA_DIR}`);
      return ids;
    }
  }

  return [];
}

// ─── Limpeza de PIN em um único doc ───────────────────────────────────────

async function clearPinForDoc(docId) {
  const result = await getDocUpdate(docId);
  if (!result) {
    return { status: "error", reason: "update não encontrado" };
  }
  const { update, base, docPath } = result;

  const ydoc = new Y.Doc();
  Y.applyUpdate(ydoc, update);
  const securityMap = ydoc.getMap("security");

  const hasPin = !!securityMap.get("passwordHash");
  if (!hasPin) {
    ydoc.destroy();
    return { status: "skip", reason: "sem PIN" };
  }

  // Remover PIN mantendo visibilityMode como "public"
  const currentMode = securityMap.get("visibilityMode");
  securityMap.delete("passwordHash");
  securityMap.set("protected", false);
  // Se o modo era "private" (acesso só com PIN), liberar para "public"
  if (!currentMode || currentMode === "private") {
    securityMap.set("visibilityMode", "public");
  }
  // "public-readonly" permanece — só remove o PIN, mantém leitura pública

  const newUpdate = Y.encodeStateAsUpdate(ydoc);
  ydoc.destroy();

  const ok = await postDocUpdate(base, docPath, newUpdate);
  if (!ok) {
    return { status: "error", reason: "falha ao escrever update" };
  }
  return { status: "cleared", previousMode: currentMode };
}

// ─── Main ──────────────────────────────────────────────────────────────────

async function main() {
  console.log("🔓 DontPad — Script de remoção de PINs\n");
  console.log(`  Y-Sweet: ${HTTP_BASE}`);
  console.log(
    `  Auth: ${AUTH_TOKEN ? "✔ token configurado" : "✖ sem token (sem auth)"}\n`,
  );

  const docIds = discoverDocIds();

  if (docIds.length === 0) {
    console.error(
      "❌ Nenhum doc ID encontrado.\n" +
        "   Use DOC_IDS=id1,id2,... para especificar manualmente, ou\n" +
        "   certifique-se que DATA_DIR aponta para o diretório de dados.\n" +
        "\n" +
        "   Em produção (Docker), rode dentro do container do app:\n" +
        "     docker exec -it <nome-do-container-app> \\\n" +
        '       sh -c \'ls /app/data | tr "\\n" ",\' | xargs -I{} \\\n' +
        "       env DOC_IDS={} node /app/scripts/clear-pins.mjs'\n" +
        "\n" +
        "   OU liste os IDs manualmente:\n" +
        "     docker exec ysweet ls /data\n" +
        "   e passe-os via: DOC_IDS=id1,id2,id3 node scripts/clear-pins.mjs",
    );
    process.exit(1);
  }

  let cleared = 0;
  let skipped = 0;
  let errors = 0;

  for (const docId of docIds) {
    process.stdout.write(`  📄 ${docId.padEnd(30)} `);
    const result = await clearPinForDoc(docId);
    if (result.status === "cleared") {
      console.log(
        `✅ PIN removido (modo anterior: ${result.previousMode || "desconhecido"})`,
      );
      cleared++;
    } else if (result.status === "skip") {
      console.log(`⏭  Sem PIN — ignorado`);
      skipped++;
    } else {
      console.log(`❌ Erro: ${result.reason}`);
      errors++;
    }
  }

  console.log(`\n─────────────────────────────────`);
  console.log(`  ✅ PINs removidos : ${cleared}`);
  console.log(`  ⏭  Sem PIN        : ${skipped}`);
  console.log(`  ❌ Erros          : ${errors}`);
  console.log(`─────────────────────────────────`);

  if (errors > 0) process.exit(1);
}

main().catch((e) => {
  console.error("Erro fatal:", e);
  process.exit(1);
});
