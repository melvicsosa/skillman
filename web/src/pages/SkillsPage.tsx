import { useCallback, useMemo, useRef, useState } from 'react'

import { api, type Skill } from '../api/client'
import { useApi } from '../hooks/useApi'
import type { ToastMessage } from '../components/molecules/Toast'
import { ToastArea } from '../components/molecules/Toast'
import type { ScopeTab } from '../components/molecules/ScopeTabs'
import type { RowFlags } from '../components/molecules/SkillRow'
import { ALL_AGENTS, Sidebar } from '../components/organisms/Sidebar'
import { TopBar } from '../components/organisms/TopBar'
import { SkillTable } from '../components/organisms/SkillTable'
import { DoctorPanel } from '../components/organisms/DoctorPanel'

const fetchAgents = () => api.agents()
const fetchSkills = () => api.skills()
const fetchProjects = () => api.projects()
const fetchDoctor = () => api.doctor()
const fetchHealth = () => api.health()

export function SkillsPage() {
  const agents = useApi(fetchAgents)
  const skills = useApi(fetchSkills)
  const projects = useApi(fetchProjects)
  const doctor = useApi(fetchDoctor)
  const health = useApi(fetchHealth)

  const [selectedAgent, setSelectedAgent] = useState<string>(ALL_AGENTS)
  const [scope, setScope] = useState<ScopeTab>('global')
  const [selectedProject, setSelectedProject] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [busySkill, setBusySkill] = useState<string | null>(null)
  const [busyAgent, setBusyAgent] = useState<string | null>(null)
  const [scanning, setScanning] = useState(false)
  const [toasts, setToasts] = useState<ToastMessage[]>([])
  const toastSeq = useRef(0)

  const toast = useCallback((title: string, body: string | undefined, tone: ToastMessage['tone'] = 'ok') => {
    const id = ++toastSeq.current
    setToasts((t) => [...t, { id, title, body, tone }])
    window.setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), tone === 'error' ? 8000 : 5000)
  }, [])

  const agentList = useMemo(() => agents.data ?? [], [agents.data])
  const agentNames = useMemo(() => Object.fromEntries(agentList.map((a) => [a.id, a.name])), [agentList])
  const enabledAgents = useMemo(() => new Set(agentList.filter((a) => a.enabled).map((a) => a.id)), [agentList])

  const allSkills = useMemo(() => skills.data ?? [], [skills.data])
  const counts = useMemo(() => {
    const c: Record<string, number> = {}
    for (const s of allSkills) c[s.agent] = (c[s.agent] ?? 0) + 1
    return c
  }, [allSkills])

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase()
    return allSkills.filter((s) => {
      if (selectedAgent === ALL_AGENTS ? !enabledAgents.has(s.agent) : s.agent !== selectedAgent) return false
      if (scope === 'global' ? s.scope !== 'global' : s.scope === 'global') return false
      if (scope === 'project' && selectedProject && s.projectRoot !== selectedProject) return false
      if (q && !s.name.toLowerCase().includes(q) && !s.description.toLowerCase().includes(q)) return false
      return true
    })
  }, [allSkills, selectedAgent, enabledAgents, scope, selectedProject, query])

  const flagIndex = useMemo(() => {
    const idx: Record<string, RowFlags> = {}
    for (const i of doctor.data?.issues ?? []) {
      if (i.kind === 'drift') {
        for (const id of i.skillIds ?? []) idx[id] = { ...idx[id], drift: `${i.message}\n${(i.details ?? []).join('\n')}` }
      } else if (i.kind === 'invalid-frontmatter' && i.skillId) {
        idx[i.skillId] = { ...idx[i.skillId], invalid: i.message }
      }
    }
    return idx
  }, [doctor.data])
  const flagsFor = useCallback((id: string) => flagIndex[id] ?? {}, [flagIndex])

  const refreshAll = useCallback(async () => {
    await Promise.all([agents.reload(), skills.reload(), projects.reload(), doctor.reload()])
  }, [agents, skills, projects, doctor])

  const onToggleSkill = async (skill: Skill, enabled: boolean) => {
    setBusySkill(skill.id)
    try {
      const updated = enabled ? await api.enableSkill(skill.id) : await api.disableSkill(skill.id)
      toast(
        enabled ? `Enabled ${updated.name}` : `Disabled ${updated.name}`,
        enabled ? `Restored to ${updated.path}` : `Moved to ${updated.quarantinePath ?? 'quarantine'}`,
      )
      await Promise.all([skills.reload(), doctor.reload()])
    } catch (err) {
      toast(`Could not ${enabled ? 'enable' : 'disable'} ${skill.name}`, errMsg(err), 'error')
    } finally {
      setBusySkill(null)
    }
  }

  const onToggleAgent = async (id: string, enabled: boolean) => {
    setBusyAgent(id)
    try {
      await api.setAgentEnabled(id, enabled)
      await agents.reload()
    } catch (err) {
      toast(`Could not update ${agentNames[id] ?? id}`, errMsg(err), 'error')
    } finally {
      setBusyAgent(null)
    }
  }

  const onScan = async () => {
    setScanning(true)
    try {
      const sum = await api.scan()
      const lines = [`${sum.found} found, ${sum.added} added, ${sum.removed} removed, ${sum.disabled} disabled`]
      if (sum.errors.length) lines.push(`${sum.errors.length} warning${sum.errors.length === 1 ? '' : 's'}: ${sum.errors[0]}`)
      toast('Scan complete', lines.join('\n'), sum.errors.length ? 'error' : 'ok')
      await refreshAll()
    } catch (err) {
      toast('Scan failed', errMsg(err), 'error')
    } finally {
      setScanning(false)
    }
  }

  const onAddProject = async () => {
    const root = window.prompt('Absolute path of the project root to register:')
    if (!root) return
    setScanning(true)
    try {
      const res = await api.addProject(root.trim())
      toast(`Registered ${res.project.name}`, `${res.scan.found} skills found in ${res.project.root}`)
      await refreshAll()
      setSelectedProject(res.project.root)
    } catch (err) {
      toast('Could not add project', errMsg(err), 'error')
    } finally {
      setScanning(false)
    }
  }

  const loadError = agents.error ?? skills.error
  const initialLoading = (agents.loading && !agents.data) || (skills.loading && !skills.data)

  return (
    <div className="app">
      <Sidebar
        agents={agentList}
        counts={counts}
        totalCount={allSkills.length}
        selected={selectedAgent}
        busyAgent={busyAgent}
        version={health.data?.version ?? ''}
        onSelect={setSelectedAgent}
        onToggle={onToggleAgent}
      />
      <main className="main">
        <TopBar
          query={query}
          onQuery={setQuery}
          scope={scope}
          onScope={setScope}
          projects={projects.data ?? []}
          selectedProject={selectedProject}
          onSelectProject={setSelectedProject}
          onAddProject={onAddProject}
          scanning={scanning}
          onScan={onScan}
        />
        <div className="content">
          {loadError ? (
            <div className="table-wrap">
              <div className="state-box is-error">
                <h3>Could not reach the skillman server</h3>
                <p>{loadError}. Is `skillman serve` running?</p>
              </div>
            </div>
          ) : initialLoading ? (
            <div className="table-wrap">
              <div className="state-box">Loading skills…</div>
            </div>
          ) : (
            <>
              <p className="summary">
                <span>
                  <strong>{visible.length}</strong> {visible.length === 1 ? 'skill' : 'skills'}
                  {selectedAgent !== ALL_AGENTS && ` for ${agentNames[selectedAgent] ?? selectedAgent}`}
                  {scope === 'project' && selectedProject && ` in ${selectedProject}`}
                </span>
                <span>{visible.filter((s) => s.state === 'disabled').length} disabled</span>
              </p>
              <SkillTable
                skills={visible}
                agentNames={agentNames}
                showAgent={selectedAgent === ALL_AGENTS}
                flagsFor={flagsFor}
                busySkill={busySkill}
                onToggle={onToggleSkill}
                emptyTitle={emptyTitle(allSkills.length, scope, projects.data?.length ?? 0, query)}
                emptyHint={emptyHint(allSkills.length, scope, projects.data?.length ?? 0, query)}
              />
              <DoctorPanel report={doctor.data} error={doctor.error} loading={doctor.loading} />
            </>
          )}
        </div>
      </main>
      <ToastArea toasts={toasts} />
    </div>
  )
}

function errMsg(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

function emptyTitle(total: number, scope: ScopeTab, projectCount: number, query: string): string {
  if (total === 0) return 'No skills cached yet'
  if (query) return `Nothing matches "${query}"`
  if (scope === 'project' && projectCount === 0) return 'No projects registered'
  return 'No skills here'
}

function emptyHint(total: number, scope: ScopeTab, projectCount: number, query: string): string {
  if (total === 0) return 'Run a scan to discover the skills installed for each agent.'
  if (query) return 'Try a different search or clear it.'
  if (scope === 'project' && projectCount === 0) return 'Use the project dropdown to add a project root, then its .agents/skills and friends are scanned.'
  return 'Pick another agent or scope, or run a scan.'
}
