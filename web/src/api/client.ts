// Thin fetch wrapper over the skillman JSON API (PLAN.md section 7).

export type Agent = {
  id: string
  name: string
  enabled: boolean
  globalDirs: string[]
  projectDirs: string[]
  exists: boolean
  skillCount: number
  readOnly: boolean
}

export type Skill = {
  id: string
  agent: string
  scope: string
  projectRoot?: string
  path: string
  name: string
  description: string
  version: string
  contentHash: string
  isSymlink: boolean
  linkTarget?: string
  state: 'enabled' | 'disabled'
  quarantinePath?: string
  readOnly: boolean
  lastSeenAt: string
}

export type Project = {
  id: number
  root: string
  name: string
  registeredAt: string
}

export type ScanSummary = {
  found: number
  added: number
  removed: number
  disabled: number
  errors: string[]
}

export type Issue = {
  kind: string
  agent?: string
  scope?: string
  skillId?: string
  name?: string
  path?: string
  message: string
  details?: string[]
  skillIds?: string[]
}

export type DoctorReport = {
  issues: Issue[]
  counts: Record<string, number>
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers ?? {}) },
  })
  if (res.status === 204) return undefined as T
  const text = await res.text()
  let body: unknown = null
  try {
    body = text ? JSON.parse(text) : null
  } catch {
    body = null
  }
  if (!res.ok) {
    const msg =
      body && typeof body === 'object' && 'error' in body
        ? String((body as { error: unknown }).error)
        : `HTTP ${res.status}`
    throw new ApiError(res.status, msg)
  }
  return body as T
}

export const api = {
  agents: () => request<Agent[]>('/api/agents'),
  setAgentEnabled: (id: string, enabled: boolean) =>
    request<Agent>(`/api/agents/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      body: JSON.stringify({ enabled }),
    }),
  skills: () => request<Skill[]>('/api/skills'),
  enableSkill: (id: string) => request<Skill>(`/api/skills/${id}/enable`, { method: 'POST' }),
  disableSkill: (id: string) => request<Skill>(`/api/skills/${id}/disable`, { method: 'POST' }),
  projects: () => request<Project[]>('/api/projects'),
  addProject: (root: string) =>
    request<{ project: Project; scan: ScanSummary }>('/api/projects', {
      method: 'POST',
      body: JSON.stringify({ root }),
    }),
  removeProject: (id: number) => request<void>(`/api/projects/${id}`, { method: 'DELETE' }),
  scan: (project?: string) =>
    request<ScanSummary>(project ? `/api/scan?project=${encodeURIComponent(project)}` : '/api/scan', {
      method: 'POST',
    }),
  doctor: () => request<DoctorReport>('/api/doctor'),
  health: () => request<{ version: string }>('/api/health'),
}
