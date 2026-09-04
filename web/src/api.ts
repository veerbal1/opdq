let csrfToken: string | null = null;

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export type Session = {
  user_id: number;
  name: string;
  clinic_id: number;
  role: string;
  csrf_token: string;
  email?: string;
};

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const headers: Record<string, string> = {};

  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }

  if (method !== "GET" && csrfToken) {
    headers["X-CSRF-Token"] = csrfToken;
  }

  const res = await fetch(path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (!res.ok) {
    if (res.status === 401) csrfToken = null;

    let message = res.statusText;
    try {
      const data = await res.json();
      if (data?.error) message = data.error;
    } catch {}
    throw new ApiError(res.status, message);
  }

  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export async function login(email: string, password: string): Promise<Session> {
  const data = await request<Session>("POST", "/api/login", {
    email,
    password,
  });
  csrfToken = data.csrf_token;
  return data;
}

export async function me(): Promise<Session> {
  const data = await request<Session>("GET", "/api/me");
  csrfToken = data.csrf_token;
  return data;
}

export async function logout(): Promise<void> {
  await request<void>("POST", "/api/logout");
  csrfToken = null;
}
