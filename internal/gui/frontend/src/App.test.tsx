// @vitest-environment jsdom
import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { CancellablePromise } from '@wailsio/runtime'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import * as ConnectionService from '../bindings/github.com/JacobBananalDev/openconnect-client/internal/gui/connectionservice'

vi.mock('../bindings/github.com/JacobBananalDev/openconnect-client/internal/gui/connectionservice', () => ({
  Status: vi.fn(),
  Connect: vi.fn(),
  Disconnect: vi.fn(),
}))

const status = vi.mocked(ConnectionService.Status)
const connect = vi.mocked(ConnectionService.Connect)
const disconnect = vi.mocked(ConnectionService.Disconnect)

beforeEach(() => {
  status.mockReset()
  connect.mockReset()
  disconnect.mockReset()
  status.mockResolvedValue({ state: 'disconnected' })
})

afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('VPN screen', () => {
  it('connects and disconnects through the bindings without exposing script options', async () => {
    const user = userEvent.setup()
    connect.mockResolvedValue({ state: 'connecting' })
    disconnect.mockResolvedValue({ state: 'disconnected' })
    render(<App />)

    await screen.findByText('disconnected')
    await user.type(screen.getByLabelText('Server'), 'vpn.example.com')
    await user.selectOptions(screen.getByLabelText('Protocol'), 'gp')
    await user.click(screen.getByRole('button', { name: 'Connect' }))
    expect(connect).toHaveBeenCalledWith({ server: 'vpn.example.com', protocol: 'gp' })
    expect(await screen.findByText('connecting')).toBeTruthy()
    expect((screen.getByRole('button', { name: 'Connect' }) as HTMLButtonElement).disabled).toBe(true)
    await user.click(screen.getByRole('button', { name: 'Disconnect' }))
    expect(disconnect).toHaveBeenCalledOnce()
    expect(await screen.findByText('disconnected')).toBeTruthy()
    expect(screen.getByText(/does not confirm that the tunnel is ready/)).toBeTruthy()
    expect(screen.getByText(/SAML\/SSO/)).toBeTruthy()
  })

  it('keeps state unknown when status fails and recovers on refresh', async () => {
    const user = userEvent.setup()
    status.mockRejectedValueOnce(new Error('socket unavailable'))
    render(<App />)

    expect(await screen.findByText('Unable to verify')).toBeTruthy()
    expect(screen.getByText('Unknown')).toBeTruthy()
    expect(screen.getByRole('alert').textContent).toContain('socket unavailable')
    expect((screen.getByRole('button', { name: 'Connect' }) as HTMLButtonElement).disabled).toBe(true)
    await user.click(screen.getByRole('button', { name: 'Refresh status' }))
    expect(await screen.findByText('disconnected')).toBeTruthy()
    expect(screen.getByText('Available')).toBeTruthy()
  })

  it('surfaces action failures and avoids assuming the connection state', async () => {
    const user = userEvent.setup()
    connect.mockRejectedValueOnce(new Error('Connection is not permitted (policy_denied)'))
    render(<App />)
    await screen.findByText('disconnected')
    await user.type(screen.getByLabelText('Server'), 'vpn.example.com')
    await user.click(screen.getByRole('button', { name: 'Connect' }))

    expect(await screen.findByText(/policy_denied/)).toBeTruthy()
    expect(screen.getByText('Unknown')).toBeTruthy()
    expect(screen.getByText('Unable to verify')).toBeTruthy()
  })

  it('does not overlap pending requests, including periodic status refreshes', async () => {
    vi.useFakeTimers()
    let finish!: (value: { state: string }) => void
    status.mockImplementationOnce(() => new CancellablePromise((resolve) => { finish = resolve }))
    render(<App />)
    expect(status).toHaveBeenCalledOnce()
    await act(async () => { vi.advanceTimersByTime(15000) })
    expect(status).toHaveBeenCalledOnce()
    expect((screen.getByRole('button', { name: 'Refresh status' }) as HTMLButtonElement).disabled).toBe(true)
    await act(async () => { finish({ state: 'disconnected' }) })
    await act(async () => { vi.advanceTimersByTime(5000) })
    expect(status).toHaveBeenCalledTimes(2)
  })

  it('prevents duplicate connect requests while an action is pending', async () => {
    const user = userEvent.setup()
    let finish!: (value: { state: string }) => void
    connect.mockImplementationOnce(() => new CancellablePromise((resolve) => { finish = resolve }))
    render(<App />)
    await screen.findByText('disconnected')
    await user.type(screen.getByLabelText('Server'), 'vpn.example.com')
    await user.click(screen.getByRole('button', { name: 'Connect' }))
    const pendingButton = screen.getByRole('button', { name: 'Connecting…' }) as HTMLButtonElement
    expect(pendingButton.disabled).toBe(true)
    await user.click(pendingButton)
    expect(connect).toHaveBeenCalledOnce()
    await act(async () => { finish({ state: 'connecting' }) })
    expect(await screen.findByText('connecting')).toBeTruthy()
  })

  it.each([
    ['connected', true, false],
    ['disconnecting', true, true],
    ['failed', false, true],
  ])('reflects %s and enables only appropriate actions', async (state, connectDisabled, disconnectDisabled) => {
    status.mockResolvedValue({ state })
    render(<App />)
    await screen.findByText(state)
    const server = screen.getByLabelText('Server') as HTMLInputElement
    if (!server.disabled) {
      await userEvent.type(server, 'vpn.example.com')
    }
    expect((screen.getByRole('button', { name: 'Connect' }) as HTMLButtonElement).disabled).toBe(connectDisabled)
    expect((screen.getByRole('button', { name: 'Disconnect' }) as HTMLButtonElement).disabled).toBe(disconnectDisabled)
  })

  it('supports keyboard navigation, placeholders, and theme switching', async () => {
    const user = userEvent.setup()
    const { container } = render(<App />)
    await screen.findByText('disconnected')
    await user.tab()
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Light theme' }))
    await user.keyboard('{Enter}')
    expect(container.querySelector('.app')?.getAttribute('data-theme')).toBe('light')
    await user.click(screen.getByRole('button', { name: 'Profiles' }))
    expect(screen.getByRole('heading', { name: 'Profiles' })).toBeTruthy()
    expect(screen.getByText('This section is not available yet.')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Profiles' }).getAttribute('aria-current')).toBe('page')
  })

  it('rejects unexpected states instead of displaying them as trusted status', async () => {
    status.mockResolvedValue({ state: 'online' })
    render(<App />)
    await waitFor(() => expect(screen.getByRole('alert').textContent).toContain('unrecognized connection state'))
    expect(screen.getByText('Unknown')).toBeTruthy()
  })
})
