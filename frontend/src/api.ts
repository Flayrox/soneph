// Petit client API : ajoute automatiquement le token d'auth (si configuré)
// à chaque requête HTTP et à l'URL WebSocket.

const TOKEN_KEY = "soneph_token";

export function getToken(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(TOKEN_KEY) || "";
}

export function setToken(token: string) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(TOKEN_KEY, token.trim());
}

export async function apiFetch(path: string, options: RequestInit = {}): Promise<Response> {
  const headers = new Headers(options.headers);
  const token = getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  return fetch(path, { ...options, headers });
}

export function wsUrl(path: string): string {
  const token = getToken();
  if (token) {
    const sep = path.includes("?") ? "&" : "?";
    return `${path}${sep}token=${encodeURIComponent(token)}`;
  }
  return path;
}
