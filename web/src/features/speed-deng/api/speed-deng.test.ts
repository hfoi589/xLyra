import { afterEach, describe, expect, it, vi } from 'vitest'
import { getSpeedDengStatus, startSpeedDeng, stopSpeedDeng } from './speed-deng'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('speed-deng api', () => {
  it('reads the protected global status endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ active: false, state: 'inactive' }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))
    vi.stubGlobal('fetch', fetchMock)

    await getSpeedDengStatus()

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/settings/speed-deng', expect.objectContaining({ credentials: 'include' }))
  })

  it('starts and stops with POST actions and no request payload', async () => {
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ active: true, state: 'active' }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    })))
    vi.stubGlobal('fetch', fetchMock)

    await startSpeedDeng()
    await stopSpeedDeng()

    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/v1/settings/speed-deng/start')
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({ method: 'POST' })
    expect(fetchMock.mock.calls[1]?.[0]).toBe('/api/v1/settings/speed-deng/stop')
    expect(fetchMock.mock.calls[1]?.[1]).toMatchObject({ method: 'POST' })
  })
})
