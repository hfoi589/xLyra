import { afterEach, describe, expect, it, vi } from 'vitest'
import { getOAuthCostShare, updateOAuthCostShareConfig } from './oauth-cost-share'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('oauth cost share api', () => {
  it('requests the isolated endpoint without model or api key filters', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('{"meta":{},"data":{}}', {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))
    vi.stubGlobal('fetch', fetchMock)

    await getOAuthCostShare({
      siteId: 'site-1',
      createdFrom: '2026-08-01T00:00:00.000Z',
      createdTo: '2026-08-02T00:00:00.000Z',
    })

    expect(fetchMock).toHaveBeenCalledOnce()
    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/v1/requests/oauth-cost-share?site_id=site-1&created_from=2026-08-01T00%3A00%3A00.000Z&created_to=2026-08-02T00%3A00%3A00.000Z')
  })

  it('persists the complete three-plan configuration', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('{}', {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))
    vi.stubGlobal('fetch', fetchMock)
    const config = {
      plus: { single_quota: 100, reset_count: 1, account_fee: 20 },
      pro_lite: { single_quota: 200, reset_count: 2, account_fee: 30 },
      pro: { single_quota: 300, reset_count: 3, account_fee: 40 },
    }

    await updateOAuthCostShareConfig(config)

    expect(fetchMock).toHaveBeenCalledOnce()
    expect(fetchMock.mock.calls[0]?.[0]).toBe('/api/v1/settings/oauth-cost-share')
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({ method: 'PUT', body: JSON.stringify(config) })
  })
})
