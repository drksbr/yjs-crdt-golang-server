import { createContext, useContext } from "react";

export type VisibilityMode = "public" | "public-readonly" | "private";

export interface DocumentSecurityContextValue {
  /** Modo de visibilidade atual do documento */
  visibilityMode: VisibilityMode;
  /** Usuário não pode editar (public-readonly sem JWT) */
  isReadOnly: boolean;
  /** Abre o modal de PIN para solicitar acesso de edição */
  requestEdit: () => void;
}

export const DocumentSecurityContext =
  createContext<DocumentSecurityContextValue>({
    visibilityMode: "public",
    isReadOnly: false,
    requestEdit: () => {},
  });

export const useDocumentSecurity = () => useContext(DocumentSecurityContext);
