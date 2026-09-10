import { useCallback, useState } from 'react'

import { api, type SettingsPatch } from '../api/client'
import { Alert } from '../components/atoms/Alert'
import { ServiceCard } from '../components/organisms/ServiceCard'
import { SettingsForm } from '../components/organisms/SettingsForm'
import { useApi } from '../hooks/useApi'
import { errMsg, type ToastFn } from '../hooks/useToast'
import { ThemeToggle } from '../components/molecules/ThemeToggle'

type Props = {
  toast: ToastFn
}

const fetchSettings = () => api.settings()
const fetchService = () => api.service()

export function SettingsPage({ toast }: Props) {
  const settings = useApi(fetchSettings)
  const service = useApi(fetchService)
  const [saving, setSaving] = useState(false)
  const [busy, setBusy] = useState<string | null>(null)
  const [restartRequired, setRestartRequired] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)

  const onSave = useCallback(
    async (patch: SettingsPatch) => {
      setSaving(true)
      try {
        const res = await api.patchSettings(patch)
        if (res.restartRequired) setRestartRequired(true)
        toast('Settings saved', patch.port !== undefined ? `Port ${res.port}${res.restartRequired ? ' (restart required)' : ''}` : undefined)
        await Promise.all([settings.reload(), service.reload()])
      } catch (err) {
        toast('Could not save settings', errMsg(err), 'error')
      } finally {
        setSaving(false)
      }
    },
    [settings, service, toast],
  )

  const run = async (key: string, fn: () => Promise<void>) => {
    setBusy(key)
    try {
      await fn()
    } catch (err) {
      toast(`Service ${key} failed`, errMsg(err), 'error')
    } finally {
      setBusy(null)
    }
  }

  const onInstall = () =>
    run('install', async () => {
      const info = await api.serviceInstall()
      toast('Service installed', `${info.label} will keep skillman running at ${info.url}`)
      await service.reload()
    })

  const onUninstall = () =>
    run('uninstall', async () => {
      await api.serviceUninstall()
      toast('Service uninstalled', 'This page keeps working until you close `skillman serve`.')
      setRestartRequired(false)
      await service.reload()
    })

  const onRestart = () =>
    run('restart', async () => {
      const res = await api.serviceRestart()
      setRestartRequired(false)
      const here = window.location.port === String(res.port)
      setNotice(
        here
          ? 'Restarting. Reload this page in a moment.'
          : `Restarting on port ${res.port}. Open ${res.url} once it is back.`,
      )
      toast('Restarting service', here ? 'Reload in a moment' : res.url)
    })

  return (
    <>
      <header className="topbar">
        <h1 className="page-title">Settings</h1>
        <span className="topbar-spacer" />
        {settings.data && (
          <span className="data-dir" title="Data directory (database, vault, quarantine, logs)">
            Data dir: <code>{settings.data.dataDir}</code>
          </span>
        )}
        <ThemeToggle />
      </header>
      <div className="content settings">
        {notice && (
          <Alert tone="info" onDismiss={() => setNotice(null)}>
            {notice}
          </Alert>
        )}
        <SettingsForm settings={settings.data} loading={settings.loading} error={settings.error} saving={saving} onSave={onSave} />
        <ServiceCard
          info={service.data}
          loading={service.loading}
          error={service.error}
          busy={busy}
          restartRequired={restartRequired}
          onInstall={onInstall}
          onUninstall={onUninstall}
          onRestart={onRestart}
        />
      </div>
    </>
  )
}
