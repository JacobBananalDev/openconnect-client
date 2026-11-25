import { useCallback, useEffect, useRef, useState } from 'react'
import * as ConnectionService from '../bindings/github.com/JacobBananalDev/openconnect-client/internal/gui/connectionservice'

type Section = 'VPN' | 'Profiles' | 'Logs' | 'Settings'
type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'disconnecting' | 'failed'
type Operation = 'status' | 'connect' | 'disconnect'

const sections: Section[] = ['VPN', 'Profiles', 'Logs', 'Settings']
const states: ConnectionState[] = ['disconnected', 'connecting', 'connected', 'disconnecting', 'failed']

function isConnectionState(state: string): state is ConnectionState {
  return states.some((knownState) => knownState === state)
}

function errorMessage(error: unknown): string {
  if (error instanceof Error && error.message) return error.message
  if (typeof error === 'string' && error) return error
  return 'The request could not be completed.'
}

function App() {
  const [section, setSection] = useState<Section>('VPN')
  const [theme, setTheme] = useState<'light' | 'dark'>('dark')
  const [server, setServer] = useState('')
  const [protocol, setProtocol] = useState('anyconnect')
  const [state, setState] = useState<ConnectionState | null>(null)
  const [statusError, setStatusError] = useState('')
  const [actionError, setActionError] = useState('')
  const [pending, setPending] = useState<Operation | null>(null)
  const [checked, setChecked] = useState(false)
  const busy = useRef(false)
  const mounted = useRef(false)

  const request = useCallback(async (operation: Operation, endpoint = '', selectedProtocol = '') => {
    if (busy.current) return
    busy.current = true
    setPending(operation)
    if (operation !== 'status') setActionError('')

    try {
      const result = operation === 'connect'
        ? await ConnectionService.Connect({ server: endpoint, protocol: selectedProtocol })
        : operation === 'disconnect'
          ? await ConnectionService.Disconnect()
          : await ConnectionService.Status()
      if (!mounted.current) return
      if (!isConnectionState(result.state)) {
        throw new Error('The daemon returned an unrecognized connection state.')
      }
      setState(result.state)
      setStatusError('')
      setChecked(true)
    } catch (error) {
      if (!mounted.current) return
      setState(null)
      setChecked(true)
      if (operation === 'status') {
        setStatusError(errorMessage(error))
      } else {
        setStatusError('Refresh status to check the daemon and connection state.')
        setActionError(errorMessage(error))
      }
    } finally {
      busy.current = false
      if (mounted.current) {
        setPending(null)
      }
    }
  }, [])

  useEffect(() => {
    mounted.current = true
    void request('status')
    const interval = window.setInterval(() => void request('status'), 5000)
    return () => {
      mounted.current = false
      window.clearInterval(interval)
    }
  }, [request])

  const canConnect = state === 'disconnected' || state === 'failed'
  const canDisconnect = state === 'connecting' || state === 'connected'
  const statusLabel = state === null ? (checked ? 'Unknown' : 'Checking') : state

  return (
    <div className="app" data-theme={theme}>
      <header className="topbar">
        <span className="brand">OpenConnect Client</span>
        <button className="theme-button" type="button" onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}>
          {theme === 'dark' ? 'Light theme' : 'Dark theme'}
        </button>
      </header>
      <div className="layout">
        <nav className="sidebar" aria-label="Main navigation">
          {sections.map((item) => (
            <button
              key={item}
              type="button"
              className={section === item ? 'nav-item active' : 'nav-item'}
              aria-current={section === item ? 'page' : undefined}
              onClick={() => setSection(item)}
            >
              {item}
            </button>
          ))}
          <p className="sidebar-note">Desktop foundation</p>
        </nav>
        <main className="content">
          {section === 'VPN' ? (
            <section aria-labelledby="vpn-title">
              <div className="heading">
                <div>
                  <p className="eyebrow">Connection</p>
                  <h1 id="vpn-title">VPN connection</h1>
                  <p className="muted">Manage the connection owned by the daemon.</p>
                </div>
                <button className="secondary-button" type="button" disabled={pending !== null} onClick={() => void request('status')}>
                  Refresh status
                </button>
              </div>
              <div className="card">
                <div className="status-row" aria-live="polite">
                  <div>
                    <p className="label">VPN state</p>
                    <p className="state">{statusLabel}</p>
                  </div>
                  <div>
                    <p className="label">Daemon</p>
                    <p>{!checked ? 'Checking' : state === null ? 'Unable to verify' : 'Available'}</p>
                  </div>
                </div>
                <p className="notice">A started VPN process does not confirm that the tunnel is ready.</p>
                {statusError && <p className="error" role="alert">Status unavailable: {statusError}</p>}
                {actionError && <p className="error" role="alert">Request failed: {actionError}</p>}
                <form onSubmit={(event) => {
                  event.preventDefault()
                  if (canConnect && pending === null && server.trim()) {
                    void request('connect', server.trim(), protocol)
                  }
                }}>
                  <div className="fields">
                    <div className="field">
                      <label htmlFor="server">Server</label>
                      <input id="server" value={server} onChange={(event) => setServer(event.target.value)} placeholder="vpn.example.com" required disabled={pending !== null || !canConnect} />
                    </div>
                    <div className="field">
                      <label htmlFor="protocol">Protocol</label>
                      <select id="protocol" value={protocol} onChange={(event) => setProtocol(event.target.value)} disabled={pending !== null || !canConnect}>
                        <option value="anyconnect">AnyConnect</option>
                        <option value="nc">Network Connect</option>
                        <option value="gp">GlobalProtect</option>
                        <option value="pulse">Pulse</option>
                      </select>
                    </div>
                  </div>
                  <div className="actions">
                    <button className="primary-button" type="submit" disabled={pending !== null || !canConnect || !server.trim()}>
                      {pending === 'connect' ? 'Connecting…' : 'Connect'}
                    </button>
                    <button className="secondary-button" type="button" disabled={pending !== null || !canDisconnect} onClick={() => void request('disconnect')}>
                      {pending === 'disconnect' ? 'Disconnecting…' : 'Disconnect'}
                    </button>
                  </div>
                </form>
                <p className="notice">SAML/SSO and other interactive authentication flows are not supported yet. Connection requests may be denied by daemon policy.</p>
              </div>
            </section>
          ) : (
            <section aria-labelledby="placeholder-title">
              <p className="eyebrow">Coming later</p>
              <h1 id="placeholder-title">{section}</h1>
              <p className="muted">This section is not available yet.</p>
            </section>
          )}
        </main>
      </div>
    </div>
  )
}

export default App
