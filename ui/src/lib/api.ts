import type { ApiError } from "../types/api";

export class ApiClientError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(path, {
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...init.headers
    },
    ...init
  });
  if (response.status === 401 && !isLoginPath()) {
    location.assign(loginPath());
  }
  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as ApiError;
    const message = typeof body.error === "string" ? body.error : body.error?.message ?? response.statusText;
    throw new ApiClientError(response.status, message);
  }
  return (await response.json()) as T;
}

function loginPath() {
  return location.pathname.startsWith("/admin/ui") ? "/admin/ui/login" : "/login";
}

function isLoginPath() {
  return location.pathname === "/login" || location.pathname === "/admin/ui/login";
}
