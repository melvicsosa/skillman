import { useCallback, useEffect, useState } from 'react'

import {
  api,
  type Agent,
  type CuratedOwner,
  type Project,
  type RegistryResult,
  type RegistrySource,
  type SearchResult,
} from '../api/client'
import { Alert } from '../components/atoms/Alert'
import { Badge } from '../components/atoms/Badge'
import { SearchInput } from '../components/atoms/SearchInput'
import { Spinner } from '../components/atoms/Spinner'
import { InstallDialog, type InstallTarget } from '../components/organisms/InstallDialog'
import { RegistryTable } from '../components/organisms/RegistryTable'
import { useApi } from '../hooks/useApi'
import { useDebounce } from '../hooks/useDebounce'
import { errMsg, type ToastFn } from '../hooks/useToast'

type Tab = 'search' | 'trending' | 'curated'

type Props = {
  agents: Agent[]
  projects: Project[]
  vaultNames: Set<string>
  toast: ToastFn
  onChanged: () => Promise<void>
}

const tabs: { id: Tab; label: string }[] = [
  { id: 'search', label: 'Search' },
  { id: 'trending', label: 'Trending' },
  { id: 'curated', label: 'Curated' },
]

const fetchTrending = () => api.trending()
const fetchCurated = () => api.curated()

export function DiscoverPage({ agents, projects, vaultNames, toast, onChanged }: Props) {
  const [tab, setTab] = useState<Tab>('search')
  const [query, setQuery] = useState('')
  const [source, setSource] = useState<RegistrySource>('')
  const [target, setTarget] = useState<InstallTarget | null>(null)
  const debounced = useDebounce(query.trim())

  const [result, setResult] = useState<SearchResult | null>(null)
  const [searching, setSearching] = useState(false)
  const [searchError, setSearchError] = useState<string | null>(null)

  useEffect(() => {
    if (!debounced) {
      setResult(null)
      setSearchError(null)
      setSearching(false)
      return
    }
    const ctrl = new AbortController()
    setSearching(true)
    api
      .search(debounced, source, ctrl.signal)
      .then((r) => {
        setResult(r)
        setSearchError(null)
      })
      .catch((err) => {
        if (ctrl.signal.aborted) return
        setSearchError(errMsg(err))
      })
      .finally(() => {
        if (!ctrl.signal.aborted) setSearching(false)
      })
    return () => ctrl.abort()
  }, [debounced, source])

  const install = useCallback((hit: RegistryResult) => setTarget({ ref: hit.ref, name: hit.name, source: hit.source }), [])

  return (
    <>
      <header className="topbar">
        <SearchInput value={query} onChange={setQuery} placeholder="Search skills.sh and GitHub" />
        <select className="select" aria-label="Source" value={source} onChange={(e) => setSource(e.target.value as RegistrySource)}>
          <option value="">All sources</option>
          <option value="skillssh">skills.sh</option>
          <option value="github">GitHub</option>
        </select>
        <div className="tabs" role="tablist" aria-label="Discover">
          {tabs.map((t) => (
            <button
              key={t.id}
              type="button"
              role="tab"
              aria-selected={tab === t.id}
              className={`tab${tab === t.id ? ' is-active' : ''}`}
              onClick={() => setTab(t.id)}
            >
              {t.label}
            </button>
          ))}
        </div>
        <span className="topbar-spacer" />
        {searching && <Spinner />}
      </header>
      <div className="content">
        {tab === 'search' && (
          <>
            {searchError && <Alert onDismiss={() => setSearchError(null)}>{searchError}</Alert>}
            {result && result.errors.length > 0 && (
              <Alert tone="warn">
                <ul className="alert-list">
                  {result.errors.map((e, i) => (
                    <li key={i}>{e}</li>
                  ))}
                </ul>
              </Alert>
            )}
            {result && (
              <p className="summary">
                <span>
                  <strong>{result.results.length}</strong> {result.results.length === 1 ? 'result' : 'results'} for "{debounced}"
                </span>
              </p>
            )}
            <RegistryTable
              hits={result?.results ?? []}
              vaultNames={vaultNames}
              onInstall={install}
              emptyTitle={debounced ? (searching ? 'Searching…' : `Nothing matches "${debounced}"`) : 'Search for a skill'}
              emptyHint={debounced ? undefined : 'Type a name or topic; results come from skills.sh and GitHub.'}
            />
          </>
        )}
        {tab === 'trending' && <TrendingTab vaultNames={vaultNames} onInstall={install} />}
        {tab === 'curated' && <CuratedTab vaultNames={vaultNames} onInstall={install} />}
      </div>
      {target && (
        <InstallDialog
          target={target}
          agents={agents}
          projects={projects}
          onSubmit={async (ref, req) => {
            const res = await api.addRef(ref, req)
            toast(`Added ${res.entries.map((e) => e.name).join(', ') || ref}`)
            await onChanged()
            return res
          }}
          onClose={() => setTarget(null)}
        />
      )}
    </>
  )
}

type ListProps = { vaultNames: Set<string>; onInstall: (hit: RegistryResult) => void }

function TrendingTab({ vaultNames, onInstall }: ListProps) {
  const trending = useApi(fetchTrending)
  if (trending.error) return <RegistryError error={trending.error} status={trending.status} onRetry={trending.reload} />
  if (trending.loading && !trending.data) return <Loading />
  return (
    <RegistryTable
      hits={trending.data ?? []}
      vaultNames={vaultNames}
      onInstall={onInstall}
      emptyTitle="Nothing trending right now"
    />
  )
}

function CuratedTab({ vaultNames, onInstall }: ListProps) {
  const curated = useApi(fetchCurated)
  if (curated.error) return <RegistryError error={curated.error} status={curated.status} onRetry={curated.reload} />
  if (curated.loading && !curated.data) return <Loading />
  const owners: CuratedOwner[] = curated.data ?? []
  if (owners.length === 0) {
    return (
      <div className="table-wrap">
        <div className="state-box">
          <h3>No curated skills</h3>
        </div>
      </div>
    )
  }
  return (
    <>
      {owners.map((o) => (
        <section key={o.owner} className="curated-group">
          <h3 className="curated-head">
            {o.owner} <Badge>{`${o.skills.length} skills`}</Badge>{' '}
            <span className="hint">{o.totalInstalls.toLocaleString()} installs</span>
          </h3>
          <RegistryTable hits={o.skills} vaultNames={vaultNames} onInstall={onInstall} emptyTitle="No skills" />
        </section>
      ))}
    </>
  )
}

function Loading() {
  return (
    <div className="table-wrap">
      <div className="state-box">Loading…</div>
    </div>
  )
}

function RegistryError({ error, status, onRetry }: { error: string; status: number | null; onRetry: () => Promise<void> }) {
  if (status === 401) {
    return (
      <Alert tone="info">
        This list needs a skills.sh token. Set <code>SKILLSSH_TOKEN</code> (or the <code>skillssh_token</code> setting) to a
        Vercel OIDC token and restart <code>skillman serve</code>. <span className="hint">({error})</span>
      </Alert>
    )
  }
  return (
    <Alert
      actions={
        <button type="button" className="btn is-ghost" onClick={() => void onRetry()}>
          Retry
        </button>
      }
    >
      {error}
    </Alert>
  )
}
