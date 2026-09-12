import { useCallback, useMemo, useState } from 'react'

import { api } from './api/client'
import { ToastArea } from './components/molecules/Toast'
import { Sidebar, VIEW_DISCOVER, VIEW_DOCTOR, VIEW_SETTINGS, VIEW_VAULT } from './components/organisms/Sidebar'
import { useApi } from './hooks/useApi'
import { useRoute } from './hooks/useRoute'
import { errMsg, useToast } from './hooks/useToast'
import { DiscoverPage } from './pages/DiscoverPage'
import { DoctorPage } from './pages/DoctorPage'
import { SettingsPage } from './pages/SettingsPage'
import { SkillsPage } from './pages/SkillsPage'
import { VaultPage } from './pages/VaultPage'

const fetchAgents = () => api.agents()
const fetchSkills = () => api.skills()
const fetchProjects = () => api.projects()
const fetchDoctor = () => api.doctor()
const fetchHealth = () => api.health()
const fetchVault = () => api.vault()

/** Shell: sidebar plus the selected view. Shared data lives here so every
 * view sees the same agents, projects and vault, and refreshes propagate. */
function App() {
  const agents = useApi(fetchAgents)
  const skills = useApi(fetchSkills)
  const projects = useApi(fetchProjects)
  const doctor = useApi(fetchDoctor)
  const health = useApi(fetchHealth)
  const vault = useApi(fetchVault)
  const { toasts, toast } = useToast()

  const [selected, setSelected] = useRoute()
  const [busyAgent, setBusyAgent] = useState<string | null>(null)
  const [redetecting, setRedetecting] = useState(false)

  const agentList = useMemo(() => agents.data ?? [], [agents.data])
  const allSkills = useMemo(() => skills.data ?? [], [skills.data])
  const counts = useMemo(() => {
    const c: Record<string, number> = {}
    for (const s of allSkills) c[s.agent] = (c[s.agent] ?? 0) + 1
    return c
  }, [allSkills])

  const onToggleAgent = async (id: string, enabled: boolean) => {
    setBusyAgent(id)
    try {
      await api.setAgentEnabled(id, enabled)
      await agents.reload()
    } catch (err) {
      const name = agentList.find((a) => a.id === id)?.name ?? id
      toast(`Could not update ${name}`, errMsg(err), 'error')
    } finally {
      setBusyAgent(null)
    }
  }

  /** Vault and registry actions change agent dirs too, so reload both. */
  const refreshLinks = useCallback(async () => {
    await Promise.all([vault.reload(), skills.reload(), doctor.reload()])
  }, [vault, skills, doctor])

  const refreshAll = useCallback(async () => {
    await Promise.all([agents.reload(), skills.reload(), projects.reload(), doctor.reload(), vault.reload()])
  }, [agents, skills, projects, doctor, vault])

  /** A global scan re-probes every agent's dirs, so newly installed agents show up. */
  const onRedetect = async () => {
    setRedetecting(true)
    try {
      const sum = await api.scan()
      const lines = [`${sum.found} skills found, ${sum.added} added, ${sum.removed} removed`]
      if (sum.errors.length) lines.push(`${sum.errors.length} warning${sum.errors.length === 1 ? '' : 's'}: ${sum.errors[0]}`)
      toast('Agents re-detected', lines.join('\n'), sum.errors.length ? 'error' : 'ok')
      await refreshAll()
    } catch (err) {
      toast('Re-detect failed', errMsg(err), 'error')
    } finally {
      setRedetecting(false)
    }
  }

  const view =
    selected === VIEW_SETTINGS ? (
      <SettingsPage
        toast={toast}
        agents={agentList}
        busyAgent={busyAgent}
        redetecting={redetecting}
        onToggleAgent={onToggleAgent}
        onRedetect={() => void onRedetect()}
      />
    ) : selected === VIEW_DOCTOR ? (
      <DoctorPage doctor={doctor} toast={toast} refreshAll={refreshAll} />
    ) : selected === VIEW_VAULT ? (
      <VaultPage
        vault={vault}
        agents={agentList}
        projects={projects.data ?? []}
        toast={toast}
        onChanged={refreshLinks}
      />
    ) : selected === VIEW_DISCOVER ? (
      <DiscoverPage
        agents={agentList}
        projects={projects.data ?? []}
        vaultNames={new Set((vault.data ?? []).map((v) => v.name))}
        toast={toast}
        onChanged={refreshLinks}
      />
    ) : (
      <SkillsPage
        agents={agents}
        skills={skills}
        projects={projects}
        doctor={doctor}
        vault={vault.data ?? []}
        selectedAgent={selected}
        toast={toast}
        refreshAll={refreshAll}
        onOpenDoctor={() => setSelected(VIEW_DOCTOR)}
      />
    )

  return (
    <div className="app">
      <Sidebar
        agents={agentList}
        counts={counts}
        totalCount={allSkills.length}
        vaultCount={vault.data?.length ?? 0}
        doctorCount={doctor.data?.issues.length ?? 0}
        selected={selected}
        version={health.data?.version ?? ''}
        onSelect={setSelected}
      />
      <main className="main">{view}</main>
      <ToastArea toasts={toasts} />
    </div>
  )
}

export default App
