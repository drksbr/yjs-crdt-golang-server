import { SignJWT, jwtVerify } from "jose";
import { cookies } from "next/headers";

const DEV_FALLBACK_SECRET = "dontpad-secret-key-change-in-production";

function getJwtSecret(): Uint8Array {
  const configuredSecret = process.env.JWT_SECRET;

  if (configuredSecret && configuredSecret !== DEV_FALLBACK_SECRET) {
    return new TextEncoder().encode(configuredSecret);
  }

  if (process.env.NODE_ENV === "production") {
    throw new Error("JWT_SECRET must be configured in production");
  }

  return new TextEncoder().encode(DEV_FALLBACK_SECRET);
}

const JWT_SECRET = getJwtSecret();

const COOKIE_NAME = "dp_auth";
const TOKEN_EXPIRY = "24h"; // Token válido por 24 horas

interface DocumentAccessPayload {
  documentId: string;
  iat: number;
  exp: number;
}

/**
 * Gera um JWT para acesso a um documento específico
 */
async function generateDocumentToken(documentId: string): Promise<string> {
  const token = await new SignJWT({ documentId })
    .setProtectedHeader({ alg: "HS256" })
    .setIssuedAt()
    .setExpirationTime(TOKEN_EXPIRY)
    .sign(JWT_SECRET);

  return token;
}

/**
 * Verifica um JWT e retorna o payload se válido
 */
async function verifyDocumentToken(
  token: string,
): Promise<DocumentAccessPayload | null> {
  try {
    const { payload } = await jwtVerify(token, JWT_SECRET);
    return payload as unknown as DocumentAccessPayload;
  } catch {
    return null;
  }
}

/**
 * Define o cookie de autenticação para um documento
 */
export async function setDocumentAuthCookie(documentId: string): Promise<void> {
  const token = await generateDocumentToken(documentId);
  const cookieStore = await cookies();

  cookieStore.set(`${COOKIE_NAME}_${documentId}`, token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "strict",
    maxAge: 60 * 60 * 24, // 24 horas
    path: "/",
  });
}

/**
 * Verifica se o usuário tem acesso a um documento
 */
export async function hasDocumentAccess(documentId: string): Promise<boolean> {
  const cookieStore = await cookies();
  const token = cookieStore.get(`${COOKIE_NAME}_${documentId}`)?.value;

  if (!token) return false;

  const payload = await verifyDocumentToken(token);
  if (!payload) return false;

  return payload.documentId === documentId;
}
