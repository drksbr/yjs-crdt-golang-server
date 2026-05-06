/**
 * Utilitários de criptografia para o DontPad
 */

// Versão do frontend
export const APP_VERSION = "V0.1.9";

/**
 * Gera um hash bcrypt do PIN fornecido.
 * Usa bcryptjs que funciona identicamente em browser e servidor Node.js.
 *
 * @param pin - O PIN a ser hasheado
 * @returns Promise com o hash bcrypt
 */
export async function hashPin(pin: string): Promise<string> {
  if (!pin || typeof pin !== "string") {
    throw new Error("PIN inválido");
  }
  throw new Error("Hash de PIN é responsabilidade do backend Go no frontend2");
}

/**
 * Verifica se um PIN corresponde a um hash bcrypt.
 * Usa bcryptjs que funciona identicamente em browser e servidor Node.js.
 *
 * @param pin - O PIN para verificar
 * @param hash - O hash bcrypt armazenado
 * @returns Promise com true se corresponde, false caso contrário
 */
export async function verifyPin(pin: string, hash: string): Promise<boolean> {
  void pin;
  void hash;
  return false;
}
