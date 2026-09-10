import { useState } from 'react'

import type { ServiceInfo } from '../../api/client'
import { Badge } from '../atoms/Badge'
import { Button } from '../atoms/Button'
import { Spinner } from '../atoms/Spinner'
import { InlineConfirm } from '../molecules/InlineConfirm'

type Props = {
  info: ServiceInfo | null
  loading: boolean
  error: string | null
  busy: string | null
  /** True after a port change that the running service has not picked up yet. */
  restartRequired: boolean
  onInstall: () => void
  onUninstall: () => void
  onRestart: () => void
}

/** Background service (launchd) state with install/uninstall/restart. */
export function ServiceCard({ info, loading, error, busy, restartRequired, onInstall, onUninstall, onRestart }: Props) {
  const [confirming, setConfirming] = useState(false)

  return (
    <section className="card">
      <header className="card-head">
        <h2>Background service</h2>
        {loading && !info && <Spinner />}
        {info && !info.supported && <Badge tone="warn">{`unsupported on ${info.platform}`}</Badge>}
        {info?.supported && info.installed && (
          <Badge tone={info.running ? 'info' : 'warn'}>{info.running ? 'running' : 'installed, stopped'}</Badge>
        )}
        {info?.supported && !info.installed && <Badge>not installed</Badge>}
        {info?.healthy && <Badge tone="info">{`answering on :${info.port}`}</Badge>}
      </header>
      <p className="card-hint">
        Keeps <code>skillman serve</code> running at login as a launchd LaunchAgent (<code>{info?.label ?? 'com.melvicsosa.skillman'}</code>).
      </p>
      {error && <p className="card-error">{error}</p>}
      {info && (
        <dl className="kv">
          <dt>Port</dt>
          <dd>
            <a href={info.url} target="_blank" rel="noreferrer">
              {info.url}
            </a>
          </dd>
          {(info.pid || info.serverPid) && (
            <>
              <dt>PID</dt>
              <dd>{info.pid || info.serverPid}</dd>
            </>
          )}
          {info.unitPath && info.installed && (
            <>
              <dt>Plist</dt>
              <dd className="path">{info.unitPath}</dd>
            </>
          )}
          {info.logDir && info.installed && (
            <>
              <dt>Logs</dt>
              <dd className="path">{info.logDir}</dd>
            </>
          )}
          {info.executable && (
            <>
              <dt>Binary</dt>
              <dd className="path">{info.executable}</dd>
            </>
          )}
        </dl>
      )}
      {restartRequired && info?.installed && (
        <p className="card-note">The port changed. Restart the service so it listens on the new port, then reload this page there.</p>
      )}
      {info?.supported && (
        <div className="card-actions">
          {!info.installed && (
            <Button variant="primary" onClick={onInstall} disabled={busy !== null} aria-busy={busy === 'install'}>
              {busy === 'install' ? <Spinner /> : null}
              Install
            </Button>
          )}
          {info.installed && (
            <Button variant={restartRequired ? 'primary' : 'default'} onClick={onRestart} disabled={busy !== null} aria-busy={busy === 'restart'}>
              {busy === 'restart' ? <Spinner /> : null}
              Restart
            </Button>
          )}
          {info.installed && !confirming && (
            <Button className="is-danger" onClick={() => setConfirming(true)} disabled={busy !== null}>
              Uninstall
            </Button>
          )}
          {info.installed && confirming && (
            <InlineConfirm
              message="Stop the service and remove the LaunchAgent?"
              confirmLabel="Uninstall"
              busy={busy === 'uninstall'}
              onConfirm={() => {
                setConfirming(false)
                onUninstall()
              }}
              onCancel={() => setConfirming(false)}
            />
          )}
        </div>
      )}
    </section>
  )
}
