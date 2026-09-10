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
  /** Present when auto-sync ran as part of the scan. */
  sync?: SyncReport
}

/** One agent-dir outcome of a vault sync. */
export type SyncLink = {
  agent: string
  scope: string
  path: string
  status: 'linked' | 'copied' | 'already' | 'conflict'
  message?: string
}

/** POST /api/sync and POST /api/vault/{name}/sync. */
export type SyncReport = {
  dryRun: boolean
  entries: { name: string; links: SyncLink[] }[]
  linked: number
  already: number
  conflicts: number
}

export type SyncRequest = {
  project?: string
  dryRun?: boolean
}

/** One on-disk copy of a drifting skill (Issue.copies for kind 'drift'). */
export type DriftCopy = {
  skillId: string
  agent: string
  scope: string
  path: string
  hash: string
  isSymlink: boolean
  vaultRef?: string
  readOnly: boolean
  repairable: boolean
  reason?: string
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
  /** Drift only: every copy that was compared. */
  copies?: DriftCopy[]
  /** Drift only: at least one copy can be overwritten from a source. */
  repairable?: boolean
  /** Drift only: vault entry with the same name, a valid repair source. */
  vaultRef?: string
}

/** POST /api/doctor/drift/{name}/repair. `from` is 'vault' or an agent id. */
export type RepairRequest = {
  from: string
  to?: string[]
  project?: string
}

export type RepairResult = {
  name: string
  from: string
  sourcePath: string
  sourceHash: string
  replaced: string[]
  skipped: { path: string; agent: string; reason: string }[]
}

/** POST /api/vault/adopt: copy an agent's skill into the vault. */
export type AdoptRequest = {
  name: string
  from: string
  link?: boolean
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
  autoSync: boolean
  links: Skill[]
}

/** POST /api/vault/export: bundle vault skills as a Claude plugin. */
export type ExportRequest = {
  outDir: string
  name: string
  version?: string
  description?: string
  skills: string[]
}

export type ExportResult = {
  dir: string
  manifest: string
  skills: string[]
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
  description?: string
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

export type RegistrySource = '' | 'skillssh' | 'github' | 'marketplace'

/** GET/PATCH /api/settings. Tokens are write-only: only their presence comes back. */
export type Settings = {
  port: number
  hasGithubToken: boolean
  hasSkillsshToken: boolean
  dataDir: string
  /** Claude plugin marketplaces ("owner/repo") searched by Discover. */
  marketplaces: string[]
  /** Set by PATCH when the port changed while the service is running. */
  restartRequired?: boolean
}

export type SettingsPatch = {
  port?: number
  githubToken?: string
  skillsshToken?: string
  marketplaces?: string[]
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
  sync: (dryRun?: boolean) =>
    request<SyncReport>('/api/sync', { method: 'POST', body: JSON.stringify({ dryRun: dryRun ?? false }) }),
  doctor: () => request<DoctorReport>('/api/doctor'),
  repairDrift: (name: string, body: RepairRequest) =>
    request<RepairResult>(`/api/doctor/drift/${encodeURIComponent(name)}/repair`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
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
  syncVault: (name: string, body: SyncRequest) =>
    request<SyncReport>(`/api/vault/${encodeURIComponent(name)}/sync`, { method: 'POST', body: JSON.stringify(body) }),
  setVaultAutoSync: (name: string, autoSync: boolean) =>
    request<VaultEntry>(`/api/vault/${encodeURIComponent(name)}`, {
      method: 'PATCH',
      body: JSON.stringify({ autoSync }),
    }),
  adoptVault: (body: AdoptRequest) =>
    request<VaultEntry>('/api/vault/adopt', { method: 'POST', body: JSON.stringify(body) }),
  exportPlugin: (body: ExportRequest) =>
    request<ExportResult>('/api/vault/export', { method: 'POST', body: JSON.stringify(body) }),
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
