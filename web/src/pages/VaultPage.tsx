import { useMemo, useState } from 'react'

import { api, type Agent, type ExportRequest, type Project, type SyncReport, type VaultEntry } from '../api/client'
import { Alert } from '../components/atoms/Alert'
import { Button } from '../components/atoms/Button'
import { SearchInput } from '../components/atoms/SearchInput'
import { Spinner } from '../components/atoms/Spinner'
import { Select } from '../components/molecules/Select'
import { ExportBar } from '../components/organisms/ExportBar'
import { InstallDialog, type InstallTarget } from '../components/organisms/InstallDialog'
import { VaultTable } from '../components/organisms/VaultTable'
import type { ApiState } from '../hooks/useApi'
import { errMsg, type ToastFn } from '../hooks/useToast'
import { ThemeToggle } from '../components/molecules/ThemeToggle'

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
  const [syncing, setSyncing] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [selected, setSelected] = useState<Set<string>>(() => new Set())
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

  const toastSync = (title: string, res: SyncReport) => {
    const conflicts = res.entries.flatMap((e) =>
      e.links.filter((l) => l.status === 'conflict').map((l) => `${e.name} → ${l.agent} ${l.path}${l.message ? `: ${l.message}` : ''}`),
    )
    toast(
      title,
      [`${res.linked} linked, ${res.already} already linked, ${res.conflicts} conflicts${res.dryRun ? ' (dry run)' : ''}`, ...conflicts].join('\n'),
      res.conflicts ? 'error' : 'ok',
    )
  }

  const onSyncAll = async () => {
    setSyncing(true)
    setError(null)
    try {
      toastSync('Sync complete', await api.sync())
      await onChanged()
    } catch (err) {
      setError(`Sync: ${errMsg(err)}`)
    } finally {
      setSyncing(false)
    }
  }

  const onSync = (entry: VaultEntry) =>
    run(entry.name, async () => {
      try {
        toastSync(`Synced ${entry.name}`, await api.syncVault(entry.name, { project: project ?? undefined }))
      } catch (err) {
        setError(`Sync ${entry.name}: ${errMsg(err)}`)
      }
    })

  const onAutoSync = (entry: VaultEntry, autoSync: boolean) =>
    run(`${entry.name}/auto-sync`, async () => {
      try {
        await api.setVaultAutoSync(entry.name, autoSync)
      } catch (err) {
        setError(`Auto-sync ${entry.name}: ${errMsg(err)}`)
      }
    })

  const onSelect = (entry: VaultEntry, on: boolean) =>
    setSelected((s) => {
      const next = new Set(s)
      if (on) next.add(entry.name)
      else next.delete(entry.name)
      return next
    })

  const onSelectAll = (on: boolean) =>
    setSelected((s) => {
      const next = new Set(s)
      for (const e of entries) {
        if (on) next.add(e.name)
        else next.delete(e.name)
      }
      return next
    })

  const selectedNames = useMemo(
    () => (vault.data ?? []).map((e) => e.name).filter((n) => selected.has(n)),
    [vault.data, selected],
  )

  const onExport = async (req: ExportRequest) => {
    setExporting(true)
    setError(null)
    try {
      const res = await api.exportPlugin(req)
      toast(`Exported ${res.skills.length} ${res.skills.length === 1 ? 'skill' : 'skills'} to ${res.dir}`, res.manifest)
      setSelected(new Set())
    } catch (err) {
      setError(`Export: ${errMsg(err)}`)
    } finally {
      setExporting(false)
    }
  }

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
        <Select
          ariaLabel="Scope"
          className="is-project"
          value={project ?? ''}
          onChange={(v) => setProject(v || null)}
          options={[
            { value: '', label: 'Global scope' },
            ...projects.map((p) => ({ value: p.root, label: p.name, hint: p.root, title: p.root })),
          ]}
        />
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
        <Button onClick={onSyncAll} disabled={syncing} aria-busy={syncing} title="Link every vault entry into every enabled agent">
          {syncing ? <Spinner /> : null}
          Sync all
        </Button>
        <ThemeToggle />
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
            {selectedNames.length > 0 && (
              <ExportBar selected={selectedNames} busy={exporting} onExport={onExport} onClear={() => setSelected(new Set())} />
            )}
            <VaultTable
              entries={entries}
              agents={columns}
              project={project}
              busy={busy}
              selected={selected}
              onSelect={onSelect}
              onSelectAll={onSelectAll}
              onToggle={onToggle}
              onUpdate={onUpdate}
              onSync={onSync}
              onAutoSync={onAutoSync}
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
