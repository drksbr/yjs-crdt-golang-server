import {
  getCollaboratorColorFromSeed,
  generateCollaboratorName,
} from "./colors";

/**
 * Returns a stable session ID that persists for the duration of the browser tab.
 * Stored in sessionStorage so each tab gets its own identity.
 */
export function getSessionId(): string {
  if (typeof sessionStorage === "undefined") return "server";
  let id = sessionStorage.getItem("dp_session_id");
  if (!id) {
    id = Math.random().toString(36).slice(2) + Date.now().toString(36);
    sessionStorage.setItem("dp_session_id", id);
  }
  return id;
}

export function getSessionIdentity(): { name: string; color: string } {
  const id = getSessionId();
  return {
    name: generateCollaboratorName(id),
    color: getCollaboratorColorFromSeed(id),
  };
}
