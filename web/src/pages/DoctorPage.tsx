import { useMemo, useState } from 'react'

import { api, type DoctorReport, type Issue } from '../api/client'
import { Button } from '../components/atoms/Button'
import { Spinner } from '../components/atoms/Spinner'
import { ThemeToggle } from '../components/molecules/ThemeToggle'
import { DoctorHelp } from '../components/organisms/DoctorHelp'
import { DoctorIssues } from '../components/organisms/DoctorIssues'
import type { ApiState } from '../hooks/useApi'
import { applyDoctorFilters, useDoctorFilters } from '../hooks/useDoctorFilters'
import { errMsg, type ToastFn } from '../hooks/useToast'

type Props = {
  doctor: ApiState<DoctorReport>
  toast: ToastFn
  /** Repairs touch agent dirs and the vault, so everything is reloaded. */
  refreshAll: () => Promise<void>
}

export function DoctorPage({ doctor, toast, refreshAll }: Props) {
  const [checkedAt, setCheckedAt] = useState<Date | null>(null)
  const total = doctor.data?.issues.length ?? 0
  const filters = useDoctorFilters()
  const issues = doctor.data?.issues
  const view = useMemo(() => applyDoctorFilters(issues ?? [], filters.filters), [issues, filters.filters])
  const shown = view.visible.length

  const onRun = async () => {
    await doctor.reload()
    setCheckedAt(new Date())
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

  const subtitle = doctor.data
    ? `${total === 0 ? 'No issues' : `${total} ${total === 1 ? 'issue' : 'issues'}`}${total > 0 && shown < total ? ` · ${shown} shown` : ''}${checkedAt ? ` · checked ${checkedAt.toLocaleTimeString()}` : ''}`
    : doctor.loading
      ? 'Checking…'
      : ''

  return (
    <>
      <header className="topbar">
        <h1 className="page-title">Doctor</h1>
        {subtitle && <span className="data-dir">{subtitle}</span>}
        <span className="topbar-spacer" />
        <Button onClick={() => void onRun()} disabled={doctor.loading} aria-busy={doctor.loading}>
          {doctor.loading ? <Spinner /> : null}
          Run again
        </Button>
        <ThemeToggle />
      </header>
      <div className="content doctor-page">
        <div className="doctor-main">
          <DoctorIssues
            report={doctor.data}
            error={doctor.error}
            loading={doctor.loading}
            filters={filters}
            view={view}
            onRepair={onRepair}
            onAdopt={onAdopt}
          />
        </div>
        <DoctorHelp />
      </div>
    </>
  )
}
