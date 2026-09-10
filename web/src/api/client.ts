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
  /** Agent-recorded telemetry (Claude Code only); 0 when unknown. */
  usageCount: number
  lastUsedAt?: string
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

export type VaultSource = {
  type: 'github' | 'skillssh' | 'local' | 'url' | 'lock' | string
  ref: string
  url?: string
  subpath?: string
  commit?: string
}

export type SpecReport = {
  valid: boolean
  issues: string[] | null
}

/** One vault entry with the agent skills that link to it (GET /api/vault). */
export type VaultEntry = {
  name: string
  path: string
  contentHash: string
  source: VaultSource
  sourceHash: string
  installedAt: string
  updatedAt: string
  convertedFrom?: string
  spec: SpecReport
  links: Skill[]
}

export type LinkResult = {
  agent: string
  scope: string
  path: string
  copied: boolean
  existing: boolean
}

export type InstallRequest = {
  agents: string[]
  project?: string
  copy?: boolean
}

export type VaultRemoveResult = {
  name: string
  unlinked: string[]
}

export type VaultUpdateResult = {
  name: string
  updated: boolean
  oldSourceHash: string
  newSourceHash: string
  copiesRefreshed: string[] | null
  warnings: string[] | null
}

export type RegistryResult = {
  registry: string
  id: string
  name: string
  source: string
  installs?: number
  url?: string
  installUrl?: string
  ref: string
}

export type SearchResult = {
  results: RegistryResult[]
  errors: string[]
}

export type CuratedOwner = {
  owner: string
  totalInstalls: number
  skills: RegistryResult[]
}

export type AddResult = {
  shape: string
  entries: Omit<VaultEntry, 'links'>[]
  links: LinkResult[]
  warnings: string[]
}

export type LockImportResult = {
  lockFile: string
  imported: string[] | null
  skipped: { name: string; reason: string }[] | null
}

export type RegistrySource = '' | 'skillssh' | 'github'

/** GET/PATCH /api/settings. Tokens are write-only: only their presence comes back. */
export type Settings = {
  port: number
  hasGithubToken: boolean
  hasSkillsshToken: boolean
  dataDir: string
  /** Set by PATCH when the port changed while the service is running. */
  restartRequired?: boolean
}

export type SettingsPatch = {
  port?: number
  githubToken?: string
  skillsshToken?: string
}

/** GET /api/service: platform service state plus what the port answers. */
export type ServiceInfo = {
  supported: boolean
  platform: string
  label: string
  installed: boolean
  running: boolean
  pid?: number
  unitPath?: string
  logDir?: string
  port: number
  url: string
  healthy: boolean
  serverPid?: number
  executable?: string
  dataDir: string
  underService: boolean
}

export type UsageReport = {
  source: string
  skills: Record<string, { usageCount: number; lastUsedAt: string }>
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
  vault: () => request<VaultEntry[]>('/api/vault'),
  removeVault: (name: string, force: boolean) =>
    request<VaultRemoveResult>(`/api/vault/${encodeURIComponent(name)}${force ? '?force=true' : ''}`, {
      method: 'DELETE',
    }),
  installVault: (name: string, body: InstallRequest) =>
    request<{ links: LinkResult[] }>(`/api/vault/${encodeURIComponent(name)}/install`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  uninstallVault: (name: string, agent: string, project?: string) =>
    request<{ removed: string[] | null }>(`/api/vault/${encodeURIComponent(name)}/uninstall`, {
      method: 'POST',
      body: JSON.stringify({ agent, project }),
    }),
  updateVault: (name: string) =>
    request<VaultUpdateResult>(`/api/vault/${encodeURIComponent(name)}/update`, { method: 'POST' }),
  search: (q: string, source: RegistrySource, signal?: AbortSignal) => {
    const params = new URLSearchParams({ q })
    if (source) params.set('source', source)
    return request<SearchResult>(`/api/registry/search?${params.toString()}`, { signal })
  },
  trending: () => request<RegistryResult[]>('/api/registry/trending'),
  curated: () => request<CuratedOwner[]>('/api/registry/curated'),
  addRef: (ref: string, body: InstallRequest) =>
    request<AddResult>('/api/registry/add', { method: 'POST', body: JSON.stringify({ ref, ...body }) }),
  importLock: () => request<LockImportResult>('/api/import-lock', { method: 'POST' }),
  settings: () => request<Settings>('/api/settings'),
  patchSettings: (body: SettingsPatch) =>
    request<Settings>('/api/settings', { method: 'PATCH', body: JSON.stringify(body) }),
  service: () => request<ServiceInfo>('/api/service'),
  serviceInstall: () => request<ServiceInfo>('/api/service/install', { method: 'POST' }),
  serviceUninstall: () => request<ServiceInfo>('/api/service/uninstall', { method: 'POST' }),
  serviceRestart: () =>
    request<{ restarting: boolean; port: number; url: string }>('/api/service/restart', { method: 'POST' }),
  usage: () => request<UsageReport>('/api/usage'),
}
