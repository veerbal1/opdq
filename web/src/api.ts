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

export type SessionItem = {
  id: number
  doctor_id: number
  doctor_name: string
  starts_at: string
  ends_at: string
  capacity: number
  delay_min: number
  avg_consult_sec: number
  status: string
  version: number
}

export type QueueItem = {
  id: number
  token_no: number
  patient_name: string
  priority: number
  state: 'waiting' | 'in_consultation' | 'absent'
}

export function listSessions(date?: string): Promise<SessionItem[]> {
  const path = date ? `/api/sessions?date=${encodeURIComponent(date)}` : '/api/sessions'
  return request<SessionItem[]>('GET', path)
}

export function queue(sessionId: number): Promise<QueueItem[]> {
  return request<QueueItem[]>('GET', `/api/sessions/${sessionId}/queue`)
}

export function createWalkIn(
  sessionId: number,
  patientName: string,
  contact: string,
  priority = 0,
): Promise<{ id: number; token_no: number; state: string; public_id: string }> {
  return request('POST', `/api/sessions/${sessionId}/walkins`, {
    patient_name: patientName,
    contact,
    priority,
  })
}

export function transition(
  appointmentId: number,
  to: string,
): Promise<{ id: number; state: string }> {
  return request('POST', `/api/appointments/${appointmentId}/transition`, { to })
}

export function setDelay(
  sessionId: number,
  delayMin: number,
  version: number,
): Promise<SessionItem> {
  return request('POST', `/api/sessions/${sessionId}/delay`, {
    delay_min: delayMin,
    version,
  })
}

export function closeSession(sessionId: number, version: number): Promise<SessionItem> {
  return request('POST', `/api/sessions/${sessionId}/close`, { version })
}

export type BoardData = {
  session_id: number
  doctor_name: string
  delay_min: number
  status: string
  now_serving: number | null
  next: number[]
}

export function board(sessionId: number): Promise<BoardData> {
  return request<BoardData>('GET', `/api/board/${sessionId}`)
}
