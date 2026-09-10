import { useEffect, useState, type FormEvent } from 'react'

import type { Settings, SettingsPatch } from '../../api/client'
import { Badge } from '../atoms/Badge'
import { Button } from '../atoms/Button'
import { Spinner } from '../atoms/Spinner'

type Props = {
  settings: Settings | null
  loading: boolean
  error: string | null
  saving: boolean
  onSave: (patch: SettingsPatch) => Promise<void>
}

/** Port and write-only registry tokens. Tokens never round-trip: the form
 * only knows whether one is set and sends a value when the user types one. */
export function SettingsForm({ settings, loading, error, saving, onSave }: Props) {
  const [port, setPort] = useState('')
  const [github, setGithub] = useState('')
  const [skillssh, setSkillssh] = useState('')
  const [marketplaces, setMarketplaces] = useState('')

  useEffect(() => {
    if (settings) {
      setPort(String(settings.port))
      setMarketplaces((settings.marketplaces ?? []).join(', '))
    }
  }, [settings])

  const portNum = Number(port)
  const portValid = Number.isInteger(portNum) && portNum >= 1024 && portNum <= 65535
  const marketplaceList = splitList(marketplaces)
  const marketplacesDirty = settings ? marketplaceList.join(',') !== (settings.marketplaces ?? []).join(',') : false
  const dirty = settings ? portNum !== settings.port || github !== '' || skillssh !== '' || marketplacesDirty : false

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!settings || !portValid || !dirty) return
    const patch: SettingsPatch = {}
    if (portNum !== settings.port) patch.port = portNum
    if (github !== '') patch.githubToken = github.trim()
    if (skillssh !== '') patch.skillsshToken = skillssh.trim()
    if (marketplacesDirty) patch.marketplaces = marketplaceList
    await onSave(patch)
    setGithub('')
    setSkillssh('')
  }

  const clearToken = (key: 'githubToken' | 'skillsshToken') => onSave({ [key]: '' })

  return (
    <form className="card" onSubmit={submit}>
      <header className="card-head">
        <h2>Server</h2>
        {loading && !settings && <Spinner />}
      </header>
      {error && <p className="card-error">{error}</p>}
      <div className="form-grid">
        <label className="field">
          <span>Port</span>
          <input
            className="input"
            type="number"
            inputMode="numeric"
            min={1024}
            max={65535}
            value={port}
            onChange={(e) => setPort(e.target.value)}
            disabled={!settings || saving}
            aria-invalid={!portValid}
          />
          <small className="field-hint">1024–65535. Applies after the server restarts.</small>
        </label>
        <label className="field">
          <span>
            GitHub token{' '}
            {settings?.hasGithubToken ? <Badge tone="info">set</Badge> : <Badge>not set</Badge>}
          </span>
          <input
            className="input"
            type="password"
            autoComplete="off"
            placeholder={settings?.hasGithubToken ? 'Enter a new token to replace it' : 'ghp_…'}
            value={github}
            onChange={(e) => setGithub(e.target.value)}
            disabled={!settings || saving}
          />
          <small className="field-hint">
            Raises the GitHub API rate limit for search and add.{' '}
            {settings?.hasGithubToken && (
              <button type="button" className="link" onClick={() => clearToken('githubToken')} disabled={saving}>
                Clear
              </button>
            )}
          </small>
        </label>
        <label className="field">
          <span>
            skills.sh token{' '}
            {settings?.hasSkillsshToken ? <Badge tone="info">set</Badge> : <Badge>not set</Badge>}
          </span>
          <input
            className="input"
            type="password"
            autoComplete="off"
            placeholder={settings?.hasSkillsshToken ? 'Enter a new token to replace it' : 'Vercel OIDC token'}
            value={skillssh}
            onChange={(e) => setSkillssh(e.target.value)}
            disabled={!settings || saving}
          />
          <small className="field-hint">
            Unlocks the skills.sh v1 API (trending, curated).{' '}
            {settings?.hasSkillsshToken && (
              <button type="button" className="link" onClick={() => clearToken('skillsshToken')} disabled={saving}>
                Clear
              </button>
            )}
          </small>
        </label>
        <label className="field">
          <span>Marketplaces</span>
          <input
            className="input"
            type="text"
            autoComplete="off"
            placeholder="anthropics/claude-plugins, owner/repo"
            value={marketplaces}
            onChange={(e) => setMarketplaces(e.target.value)}
            disabled={!settings || saving}
          />
          <small className="field-hint">Comma-separated Claude plugin marketplaces (owner/repo) searched by Discover.</small>
        </label>
      </div>
      <p className="card-hint">
        Environment variables <code>GITHUB_TOKEN</code> and <code>SKILLSSH_TOKEN</code> are used when the stored token is empty.
        Values are never shown again once saved.
      </p>
      <div className="card-actions">
        <Button variant="primary" type="submit" disabled={!settings || saving || !dirty || !portValid} aria-busy={saving}>
          {saving ? <Spinner /> : null}
          Save
        </Button>
      </div>
    </form>
  )
}

function splitList(value: string): string[] {
  return value
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
}
