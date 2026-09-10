import { useCallback, useMemo, useState } from 'react'

import { api, type Agent, type DoctorReport, type Issue, type Project, type Skill, type VaultEntry } from '../api/client'
import type { ApiState } from '../hooks/useApi'
import { errMsg, type ToastFn } from '../hooks/useToast'
import type { ScopeTab } from '../components/molecules/ScopeTabs'
import type { RowFlags } from '../components/molecules/SkillRow'
import { ALL_AGENTS } from '../components/organisms/Sidebar'
import { TopBar, type SkillSort } from '../components/organisms/TopBar'
import { SkillTable } from '../components/organisms/SkillTable'
import { DoctorPanel } from '../components/organisms/DoctorPanel'

type Props = {
  agents: ApiState<Agent[]>
  skills: ApiState<Skill[]>
  projects: ApiState<Project[]>
  doctor: ApiState<DoctorReport>
  vault: VaultEntry[]
  selectedAgent: string
  toast: ToastFn
  refreshAll: () => Promise<void>
}

export function SkillsPage({ agents, skills, projects, doctor, vault, selectedAgent, toast, refreshAll }: Props) {
  const [scope, setScope] = useState<ScopeTab>('global')
  const [selectedProject, setSelectedProject] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [busySkill, setBusySkill] = useState<string | null>(null)
  const [scanning, setScanning] = useState(false)
  const [sort, setSort] = useState<SkillSort>('name')

  const agentList = useMemo(() => agents.data ?? [], [agents.data])
  const agentNames = useMemo(() => Object.fromEntries(agentList.map((a) => [a.id, a.name])), [agentList])
  const enabledAgents = useMemo(() => new Set(agentList.filter((a) => a.enabled).map((a) => a.id)), [agentList])
  const allSkills = useMemo(() => skills.data ?? [], [skills.data])

  const hasUsage = useMemo(() => allSkills.some((s) => s.usageCount > 0), [allSkills])

  // A project removed elsewhere must not leave a stale, invisible filter.
  const projectFilter = useMemo(
    () => (selectedProject && (projects.data ?? []).some((p) => p.root === selectedProject) ? selectedProject : null),
    [selectedProject, projects.data],
  )

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase()
    const list = allSkills.filter((s) => {
      if (selectedAgent === ALL_AGENTS ? !enabledAgents.has(s.agent) : s.agent !== selectedAgent) return false
      if (scope === 'global' ? s.scope !== 'global' : s.scope === 'global') return false
      if (scope === 'project' && projectFilter && s.projectRoot !== projectFilter) return false
      if (q && !s.name.toLowerCase().includes(q) && !s.description.toLowerCase().includes(q)) return false
      return true
    })
    if (sort === 'used') {
      // Stable: the API order (by name) breaks ties.
      list.sort((a, b) => b.usageCount - a.usageCount)
    }
    return list
  }, [allSkills, selectedAgent, enabledAgents, scope, projectFilter, query, sort])

  const flagIndex = useMemo(() => {
    const idx: Record<string, RowFlags> = {}
    for (const i of doctor.data?.issues ?? []) {
      if (i.kind === 'drift') {
        for (const id of i.skillIds ?? []) idx[id] = { ...idx[id], drift: `${i.message}\n${(i.details ?? []).join('\n')}` }
      } else if (i.kind === 'invalid-frontmatter' && i.skillId) {
        idx[i.skillId] = { ...idx[i.skillId], invalid: i.message }
      }
    }
    // The skills API does not expose vaultRef; derive it from the vault links.
    for (const v of vault) for (const l of v.links) idx[l.id] = { ...idx[l.id], vault: v.name }
    return idx
  }, [doctor.data, vault])
  const flagsFor = useCallback((id: string) => flagIndex[id] ?? {}, [flagIndex])

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

  /** Errors propagate so the project popover can show them inline. */
  const onAddProject = async (root: string) => {
    const res = await api.addProject(root)
    toast(`Registered ${res.project.name}`, `${res.scan.found} skills found in ${res.project.root}`)
    await refreshAll()
    setSelectedProject(res.project.root)
  }

  const onRemoveProject = async (project: Project) => {
    await api.removeProject(project.id)
    toast(`Removed ${project.name}`, 'Its cached skills were dropped; files on disk are untouched.')
    if (selectedProject === project.root) setSelectedProject(null)
    await refreshAll()
  }

  const onRepair = async (issue: Issue, from: string) => {
    const name = issue.name ?? ''
    const project = issue.scope?.startsWith('project:') ? issue.scope.slice('project:'.length) : undefined
    try {
      const res = await api.repairDrift(name, { from, project })
      const lines = [
        `${res.replaced.length} ${res.replaced.length === 1 ? 'copy' : 'copies'} replaced from ${res.from} (${res.sourceHash.slice(0, 8)})`,
        ...res.skipped.map((s) => `skipped ${s.agent} ${s.path}: ${s.reason}`),
      ]
      toast(`Repaired ${res.name}`, lines.join('\n'), res.skipped.length ? 'error' : 'ok')
      await refreshAll()
    } catch (err) {
      toast(`Could not repair ${name}`, errMsg(err), 'error')
    }
  }

  const onAdopt = async (issue: Issue, from: string) => {
    const name = issue.name ?? ''
    try {
      const entry = await api.adoptVault({ name, from })
      toast(`Adopted ${entry.name} into the vault`, `From ${from} · ${entry.path}`)
      await refreshAll()
    } catch (err) {
      toast(`Could not adopt ${name}`, errMsg(err), 'error')
    }
  }

  const loadError = agents.error ?? skills.error
  const initialLoading = (agents.loading && !agents.data) || (skills.loading && !skills.data)

  return (
    <>
      <TopBar
        query={query}
        onQuery={setQuery}
        scope={scope}
        onScope={setScope}
        projects={projects.data ?? []}
        selectedProject={projectFilter}
        onSelectProject={setSelectedProject}
        onAddProject={onAddProject}
        onRemoveProject={onRemoveProject}
        sort={sort}
        onSort={setSort}
        sortable={hasUsage}
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
                {scope === 'project' && projectFilter && ` in ${projectFilter}`}
              </span>
              <span>{visible.filter((s) => s.state === 'disabled').length} disabled</span>
            </p>
            <SkillTable
              skills={visible}
              agentNames={agentNames}
              showAgent={selectedAgent === ALL_AGENTS}
              showUsage={hasUsage}
              flagsFor={flagsFor}
              busySkill={busySkill}
              onToggle={onToggleSkill}
              emptyTitle={emptyTitle(allSkills.length, scope, projects.data?.length ?? 0, query)}
              emptyHint={emptyHint(allSkills.length, scope, projects.data?.length ?? 0, query)}
            />
            <DoctorPanel
              report={doctor.data}
              error={doctor.error}
              loading={doctor.loading}
              onRepair={onRepair}
              onAdopt={onAdopt}
            />
          </>
        )}
      </div>
    </>
  )
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
  if (scope === 'project' && projectCount === 0) return 'Use Add project to register a project root, then its .agents/skills and friends are scanned.'
  return 'Pick another agent or scope, or run a scan.'
}
