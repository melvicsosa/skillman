import { useEffect, useState } from 'react'
import './App.css'

type Agent = {
  id: string
  name: string
  globalDirs: string[]
  projectDirs: string[]
  exists: boolean
}

type LoadState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'ready'; agents: Agent[] }

function App() {
  const [state, setState] = useState<LoadState>({ status: 'loading' })

  useEffect(() => {
    let cancelled = false
    fetch('/api/agents')
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json() as Promise<Agent[]>
      })
      .then((agents) => {
        if (!cancelled) setState({ status: 'ready', agents })
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setState({ status: 'error', message: err instanceof Error ? err.message : String(err) })
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <main className="app">
      <header>
        <h1>skillman</h1>
        <p className="subtitle">Cross-agent AI skill manager</p>
      </header>

      {state.status === 'loading' && <p>Loading agents…</p>}
      {state.status === 'error' && <p className="error">Failed to load agents: {state.message}</p>}
      {state.status === 'ready' && (
        <ul className="agents">
          {state.agents.map((agent) => (
            <li key={agent.id} className="agent">
              <div className="agent-head">
                <span className="agent-name">{agent.name}</span>
                <span className={`badge ${agent.exists ? 'badge-ok' : 'badge-missing'}`}>
                  {agent.exists ? 'exists' : 'missing'}
                </span>
              </div>
              <code className="agent-dir">{agent.globalDirs[0]}</code>
            </li>
          ))}
        </ul>
      )}
    </main>
  )
}

export default App
