import bcrypt from "bcryptjs";

/**
 * Utilitários de criptografia para o DontPad
 *
 * IMPORTANTE: Usa bcryptjs para máxima compatibilidade entre browser e servidor
 * Funciona identicamente em ambos os ambientes
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
  try {
    if (!pin || typeof pin !== "string") {
      throw new Error("PIN inválido");
    }

    // bcryptjs funciona em browser e servidor
    const hash = await bcrypt.hash(pin, 10);
    return hash;
  } catch (error) {
    console.error("[crypto] Erro ao fazer hash do PIN:", error);
    throw error;
  }
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
  try {
    if (!pin || typeof pin !== "string") return false;

    if (!hash || typeof hash !== "string") return false;

    const isValid = await bcrypt.compare(pin, hash);
    return isValid;
  } catch (error) {
    console.error("[crypto] Erro ao verificar PIN:", error);
    // Retorna false em caso de erro ao invés de lançar exceção
    return false;
  }
}
