import { useMemo, useState } from 'react'

import { api, type Agent, type Project, type VaultEntry } from '../api/client'
import { Alert } from '../components/atoms/Alert'
import { Button } from '../components/atoms/Button'
import { SearchInput } from '../components/atoms/SearchInput'
import { Spinner } from '../components/atoms/Spinner'
import { InstallDialog, type InstallTarget } from '../components/organisms/InstallDialog'
import { VaultTable } from '../components/organisms/VaultTable'
import type { ApiState } from '../hooks/useApi'
import { errMsg, type ToastFn } from '../hooks/useToast'

type Props = {
  vault: ApiState<VaultEntry[]>
  agents: Agent[]
  projects: Project[]
  toast: ToastFn
  /** Called after any change that touches agent dirs or the vault. */
  onChanged: () => Promise<void>
}

export function VaultPage({ vault, agents, projects, toast, onChanged }: Props) {
  const [query, setQuery] = useState('')
  const [project, setProject] = useState<string | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [importing, setImporting] = useState(false)
  const [ref, setRef] = useState('')
  const [target, setTarget] = useState<InstallTarget | null>(null)
  const [error, setError] = useState<string | null>(null)

  const columns = useMemo(() => agents.filter((a) => a.enabled), [agents])
  const entries = useMemo(() => {
    const q = query.trim().toLowerCase()
    return (vault.data ?? []).filter((e) => !q || e.name.toLowerCase().includes(q) || e.source.ref.toLowerCase().includes(q))
  }, [vault.data, query])

  const run = async (key: string, fn: () => Promise<void>) => {
    setBusy(key)
    setError(null)
    try {
      await fn()
      await onChanged()
    } finally {
      setBusy(null)
    }
  }

  const onToggle = (entry: VaultEntry, agent: Agent, installed: boolean) =>
    run(`${entry.name}/${agent.id}`, async () => {
      try {
        if (installed) {
          await api.installVault(entry.name, { agents: [agent.id], project: project ?? undefined })
        } else {
          await api.uninstallVault(entry.name, agent.id, project ?? undefined)
        }
      } catch (err) {
        setError(`${installed ? 'Install' : 'Uninstall'} ${entry.name} for ${agent.name}: ${errMsg(err)}`)
      }
    })

  const onUpdate = (entry: VaultEntry) =>
    run(entry.name, async () => {
      try {
        const res = await api.updateVault(entry.name)
        const warn = res.warnings ?? []
        toast(
          res.updated ? `Updated ${res.name}` : `${res.name} is up to date`,
          [
            res.updated ? `${res.oldSourceHash.slice(0, 8)} → ${res.newSourceHash.slice(0, 8)}` : '',
            res.copiesRefreshed?.length ? `${res.copiesRefreshed.length} copies refreshed` : '',
            ...warn,
          ]
            .filter(Boolean)
            .join('\n'),
          warn.length ? 'error' : 'ok',
        )
      } catch (err) {
        setError(`Update ${entry.name}: ${errMsg(err)}`)
      }
    })

  // Rethrows so the row can catch the 409 and offer "Remove anyway".
  const onRemove = (entry: VaultEntry, force: boolean) =>
    run(entry.name, async () => {
      const res = await api.removeVault(entry.name, force)
      toast(`Removed ${res.name}`, res.unlinked.length ? `Unlinked ${res.unlinked.length} install${res.unlinked.length === 1 ? '' : 's'}` : undefined)
    })

  const onImportLock = async () => {
    setImporting(true)
    setError(null)
    try {
      const res = await api.importLock()
      const imported = res.imported ?? []
      const skipped = res.skipped ?? []
      toast(
        `Imported ${imported.length} from lock file`,
        [res.lockFile, ...skipped.map((s) => `skipped ${s.name}: ${s.reason}`)].join('\n'),
        skipped.length ? 'error' : 'ok',
      )
      await onChanged()
    } catch (err) {
      setError(`Import lock file: ${errMsg(err)}`)
    } finally {
      setImporting(false)
    }
  }

  const openAdd = () => {
    const r = ref.trim()
    if (r) setTarget({ ref: r })
  }

  return (
    <>
      <header className="topbar">
        <SearchInput value={query} onChange={setQuery} placeholder="Filter vault" />
        <select className="select" aria-label="Scope" value={project ?? ''} onChange={(e) => setProject(e.target.value || null)}>
          <option value="">Global scope</option>
          {projects.map((p) => (
            <option key={p.id} value={p.root} title={p.root}>
              {p.name} — {p.root}
            </option>
          ))}
        </select>
        <span className="topbar-spacer" />
        <form
          className="ref-form"
          onSubmit={(e) => {
            e.preventDefault()
            openAdd()
          }}
        >
          <input
            className="input"
            value={ref}
            onChange={(e) => setRef(e.target.value)}
            placeholder="owner/repo, owner/repo/skill, URL or path"
            aria-label="Add by ref"
          />
          <Button type="submit" disabled={!ref.trim()}>
            Add by ref
          </Button>
        </form>
        <Button onClick={onImportLock} disabled={importing} aria-busy={importing}>
          {importing ? <Spinner /> : null}
          Import lock file
        </Button>
      </header>
      <div className="content">
        {error && <Alert onDismiss={() => setError(null)}>{error}</Alert>}
        {vault.error ? (
          <Alert
            actions={
              <Button variant="ghost" onClick={() => void vault.reload()}>
                Retry
              </Button>
            }
          >
            Could not load the vault: {vault.error}
          </Alert>
        ) : vault.loading && !vault.data ? (
          <div className="table-wrap">
            <div className="state-box">Loading vault…</div>
          </div>
        ) : (
          <>
            <p className="summary">
              <span>
                <strong>{entries.length}</strong> {entries.length === 1 ? 'entry' : 'entries'}
                {project ? ` · installs in ${project}` : ' · global installs'}
              </span>
              <span>{columns.length === 0 ? 'No enabled agents' : `${columns.length} enabled agents`}</span>
            </p>
            <VaultTable
              entries={entries}
              agents={columns}
              project={project}
              busy={busy}
              onToggle={onToggle}
              onUpdate={onUpdate}
              onRemove={onRemove}
            />
          </>
        )}
      </div>
      {target && (
        <InstallDialog
          target={target}
          agents={agents}
          projects={projects}
          onSubmit={async (r, req) => {
            const res = await api.addRef(r, req)
            setRef('')
            await onChanged()
            return res
          }}
          onClose={() => setTarget(null)}
        />
      )}
    </>
  )
}
