import * as Y from "yjs";

/**
 * Converte uma connection string Y-Sweet para URL HTTP interna.
 * ys://token@host:port  → http://host:port
 * yss://token@host:port → https://host:port
 */
export function ysToHttpUrl(conn: string): string {
  return conn
    .replace(/^yss?:\/\/[^@]*@/, "http://")
    .replace(/^yss:\/\//, "https://")
    .replace(/^ys:\/\//, "http://");
}

/**
 * Extrai o token de autenticação da connection string.
 * Fallback para a variável de ambiente YSWEET_AUTH_TOKEN.
 */
export function extractAuthToken(conn: string): string | undefined {
  const match = conn.match(/^ys[s]?:\/\/([^@]+)@/);
  return match
    ? decodeURIComponent(match[1])
    : process.env.YSWEET_AUTH_TOKEN || undefined;
}

/**
 * Busca o estado atual de um documento via HTTP interno do Y-Sweet.
 * Segue o fluxo recomendado: POST /doc/:id/auth → recebe baseUrl → GET {baseUrl}/as-update.
 * Fallback para localhost:8080 se o host configurado falhar.
 */
export async function getDocUpdate(docId: string): Promise<Uint8Array | null> {
  const conn = process.env.CONNECTION_STRING || "ys://ysweet:8080";
  const httpBase = ysToHttpUrl(conn);
  const authToken = extractAuthToken(conn);

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}),
  };

  const bases = [httpBase, "http://localhost:8080"];

  for (const base of bases) {
    try {
      // Passo 1: autenticar para obter o path doc-específico
      const authRes = await fetch(`${base}/doc/${docId}/auth`, {
        method: "POST",
        headers,
        body: JSON.stringify({ authorization: "full" }),
      });
      if (!authRes.ok) continue;

      const { baseUrl } = await authRes.json();
      if (!baseUrl) continue;

      // Passo 2: o baseUrl usa URL_PREFIX externo (ex: https://pad.exemplo.com/doc/abc)
      // Extraímos só o pathname e usamos o host interno para evitar requisição externa
      const docPath = new URL(baseUrl).pathname;
      const updateRes = await fetch(`${base}${docPath}/as-update`, { headers });
      if (!updateRes.ok) continue;

      return new Uint8Array(await updateRes.arrayBuffer());
    } catch {
      // tenta próxima base
    }
  }
  return null;
}

/**
 * Verifica se um documento tem PIN configurado consultando o Y-Sweet via HTTP interno.
 */
export async function isDocumentProtected(docId: string): Promise<boolean> {
  const { isProtected } = await getDocumentSecurity(docId);
  return isProtected;
}

/**
 * Lê o modo de visibilidade e se o documento está protegido via HTTP interno do Y-Sweet.
 */
export async function getDocumentSecurity(docId: string): Promise<{
  isProtected: boolean;
  visibilityMode: "public" | "public-readonly" | "private";
}> {
  try {
    const update = await getDocUpdate(docId);
    if (!update || update.byteLength === 0)
      return { isProtected: false, visibilityMode: "public" };
    const ydoc = new Y.Doc();
    try {
      Y.applyUpdateV2(ydoc, update);
    } catch {
      Y.applyUpdate(ydoc, update);
    }
    const securityMap = ydoc.getMap("security");
    const hasPin = !!securityMap.get("passwordHash");
    const storedMode = securityMap.get("visibilityMode") as string | undefined;
    ydoc.destroy();

    let visibilityMode: "public" | "public-readonly" | "private";
    if (storedMode === "public-readonly") visibilityMode = "public-readonly";
    else if (storedMode === "private" || (hasPin && !storedMode))
      visibilityMode = "private"; // backward compat
    else visibilityMode = "public";

    return { isProtected: visibilityMode === "private", visibilityMode };
  } catch {
    return { isProtected: false, visibilityMode: "public" };
  }
}
