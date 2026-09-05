import { describe, expect, it } from 'vitest'
import {
  buildCostSharePieSlices,
  costShareWarning,
  costShareQueryFromSearch,
  costShareConfigToDraft,
  costShareQueryParams,
  draftToCostShareConfig,
} from './oauth-cost-share-utils'
import type { OAuthCostShareConfig, OAuthCostShareData, OAuthCostShareResponse } from './types'

const config: OAuthCostShareConfig = {
  plus: { single_quota: 100, reset_count: 1, account_fee: 20 },
  pro_lite: { single_quota: 200, reset_count: 2, account_fee: 30 },
  pro: { single_quota: 300, reset_count: 3, account_fee: 40 },
}

const data: OAuthCostShareData = {
  supported: true,
  site_id: 'site-1',
  site_label: 'Codex',
  plan_type: 'plus',
  single_quota: 100,
  reset_count: 1,
  total_quota: 200,
  account_fee: 20,
  total_usage_cost: 50,
  total_usage_ratio: 0.25,
  allocated_cost: 5,
  unallocated_cost: 15,
  over_quota: false,
  items: [
    { name: 'Wilson', usage_cost: 40, usage_share: 0.2, allocated_cost: 4 },
    { name: 'Other', usage_cost: 10, usage_share: 0.05, allocated_cost: 1 },
  ],
}

describe('costShareQueryParams', () => {
  it('sends only one site and the active date range', () => {
    const params = costShareQueryParams({
      siteId: 'site-1',
      createdFrom: '2026-08-01T00:00:00.000Z',
      createdTo: '2026-08-02T00:00:00.000Z',
    })

    expect(params.toString()).toBe('site_id=site-1&created_from=2026-08-01T00%3A00%3A00.000Z&created_to=2026-08-02T00%3A00%3A00.000Z')
    expect(params.has('model_key')).toBe(false)
    expect(params.has('api_key_id')).toBe(false)
  })
})

describe('costShareQueryFromSearch', () => {
  it('returns a query only when the URL selects exactly one site', () => {
    expect(costShareQueryFromSearch('?site_id=site-1&created_from=2026-08-01T00%3A00%3A00.000Z')).toEqual({
      siteId: 'site-1',
      createdFrom: '2026-08-01T00:00:00.000Z',
      createdTo: undefined,
    })
    expect(costShareQueryFromSearch('?site_id=site-1&site_id=site-2')).toBeNull()
  })
})

describe('cost share config conversion', () => {
  it('round-trips numeric settings through editable string drafts', () => {
    const draft = costShareConfigToDraft(config)
    expect(draft.plus).toEqual({ single_quota: '100', reset_count: '1', account_fee: '20' })
    expect(draftToCostShareConfig(draft)).toEqual(config)
  })
})

describe('buildCostSharePieSlices', () => {
  it('keeps allocation values and adds the unallocated fee slice', () => {
    expect(buildCostSharePieSlices(data)).toEqual([
      { name: 'Wilson', value: 4, usage_cost: 40, usage_share: 0.2, allocated_cost: 4, is_unallocated: false },
      { name: 'Other', value: 1, usage_cost: 10, usage_share: 0.05, allocated_cost: 1, is_unallocated: false },
      { name: '未分摊费用', value: 15, usage_cost: 0, usage_share: 0.75, allocated_cost: 15, is_unallocated: true },
    ])
  })

  it('does not add a remaining slice after quota is exceeded', () => {
    expect(buildCostSharePieSlices({ ...data, over_quota: true, unallocated_cost: 0 })).toHaveLength(2)
  })

  it('accepts a localized label for the unallocated slice', () => {
    expect(buildCostSharePieSlices(data, 'Unallocated cost').at(-1)?.name).toBe('Unallocated cost')
  })
})

describe('costShareWarning', () => {
  it('keeps the speed-deng warning visible even when the result has no items', () => {
    const response = {
      meta: {
        range_start: '2026-08-01T00:00:00.000Z',
        range_end: '2026-08-02T00:00:00.000Z',
        timezone: 'UTC',
        currency: 'USD',
        request_count: 0,
        missing_cost_requests: 0,
        speed_deng_warning: 'custom event table unavailable',
      },
      data: { ...data, items: [] },
    } satisfies OAuthCostShareResponse

    expect(costShareWarning(response)).toBe('custom event table unavailable')
  })
})
