import { describe, expect, it } from 'vitest'
import {
  analyticsSearchParams,
  buildBarChartRows,
  buildScatterChartPoints,
  buildScatterTimeAxis,
  buildSankeyChartData,
  type AnalyticsBarData,
  type AnalyticsFilterState,
} from '@/features/requests/lib/analytics-utils'

const filters: AnalyticsFilterState = {
  createdFrom: '2026-08-01T00:00:00.000Z',
  createdTo: '2026-08-05T00:00:00.000Z',
  modelKeys: ['gpt-5', 'claude-3-7'],
  siteIds: ['site-a'],
  apiKeyIds: ['key-a', 'key-b'],
  currency: 'USD',
}

describe('analyticsSearchParams', () => {
  it('encodes repeated dimensions without exposing a status filter', () => {
    const params = analyticsSearchParams('bar', filters)

    expect(params.get('view')).toBe('bar')
    expect(params.getAll('model_key')).toEqual(['claude-3-7', 'gpt-5'])
    expect(params.getAll('api_key_id')).toEqual(['key-a', 'key-b'])
    expect(params.has('success')).toBe(false)
    expect(params.get('currency')).toBe('USD')
  })
})

describe('buildBarChartRows', () => {
  it('aggregates the complete range into one model bar with stacked downstream keys', () => {
    const data: AnalyticsBarData = {
      bucket_unit: 'day',
      series: [
        { model_key: 'gpt-5', model_label: 'GPT-5', api_key_id: 'key-a', api_key_label: 'A', is_other: false },
        { model_key: 'gpt-5', model_label: 'GPT-5', api_key_id: 'key-b', api_key_label: 'B', is_other: false },
        { model_key: 'claude-3-7', model_label: 'Claude 3.7', api_key_id: 'key-a', api_key_label: 'A', is_other: false },
      ],
      points: [
        {
          bucket_start: '2026-08-01T00:00:00Z',
          bucket_end: '2026-08-02T00:00:00Z',
          groups: [
            {
              model_key: 'gpt-5',
              model_label: 'GPT-5',
              total_cost: 3,
              segments: [
                { api_key_id: 'key-a', api_key_label: 'A', cost: 1, is_other: false },
                { api_key_id: 'key-b', api_key_label: 'B', cost: 2, is_other: false },
              ],
            },
            {
              model_key: 'claude-3-7',
              model_label: 'Claude 3.7',
              total_cost: 4,
              segments: [{ api_key_id: 'key-a', api_key_label: 'A', cost: 4, is_other: false }],
            },
          ],
        },
        {
          bucket_start: '2026-08-02T00:00:00Z',
          bucket_end: '2026-08-03T00:00:00Z',
          groups: [{
            model_key: 'gpt-5',
            model_label: 'GPT-5',
            total_cost: 5,
            segments: [{ api_key_id: 'key-a', api_key_label: 'A', cost: 5, is_other: false }],
          }],
        },
      ],
    }

    const first = buildBarChartRows(data)
    const second = buildBarChartRows(data)
    expect(first.series).toEqual(second.series)
    expect(first.series[0].stackId).toBe(first.series[1].stackId)
    expect(first.rows).toHaveLength(2)
    expect(first.rows.map((row) => row.model_key)).toEqual(['gpt-5', 'claude-3-7'])
    expect(first.rows[0].model_label).toBe('GPT-5')
    expect(first.rows[0][first.series[0].dataKey]).toBe(6)
    expect(first.rows[0][first.series[1].dataKey]).toBe(2)
    expect(first.rows[1][first.series[2].dataKey]).toBe(4)
  })
})

describe('buildSankeyChartData', () => {
  it('maps API node IDs to the indexes required by Recharts', () => {
    const result = buildSankeyChartData({
      nodes: [
        { id: 'site:1', label: 'Site A', type: 'site', is_other: false },
        { id: 'model:gpt-5', label: 'GPT-5', type: 'model', is_other: false },
      ],
      links: [{ source: 'site:1', target: 'model:gpt-5', value: 42 }],
      total_tokens: 42,
    })

    expect(result.nodes.map((node) => node.name)).toEqual(['Site A', 'GPT-5'])
    expect(result.nodes.map((node) => node.label)).toEqual(['Site A', 'GPT-5'])
    expect(result.nodes.map((node) => node.type)).toEqual(['site', 'model'])
    expect(result.links).toEqual([{ source: 0, target: 1, value: 42 }])
  })
})

describe('buildScatterChartPoints', () => {
  it('uses the fifteen-minute bucket start as the X coordinate and keeps total cost on Y', () => {
    const result = buildScatterChartPoints([{
      bucket_start: '2026-08-05T10:15:00.000Z',
      bucket_end: '2026-08-05T10:30:00.000Z',
      request_count: 3,
      total_tokens: 4096,
      total_cost: 1.25,
      currency: 'USD',
    }])

    expect(result).toEqual([{
      bucket_start: '2026-08-05T10:15:00.000Z',
      bucket_end: '2026-08-05T10:30:00.000Z',
      request_count: 3,
      total_tokens: 4096,
      total_cost: 1.25,
      currency: 'USD',
      bucket_timestamp: Date.parse('2026-08-05T10:15:00.000Z'),
    }])
  })

  it('keeps the X axis bound to the selected range and creates hourly ticks', () => {
    const points = buildScatterChartPoints([{
      bucket_start: '2026-08-05T10:15:00.000Z',
      bucket_end: '2026-08-05T10:30:00.000Z',
      request_count: 3,
      total_tokens: 4096,
      total_cost: 1.25,
      currency: 'USD',
    }])

    const axis = buildScatterTimeAxis(
      points,
      '2026-08-05T10:00:00.000Z',
      '2026-08-05T13:00:00.000Z',
    )

    expect(axis.domain).toEqual([
      Date.parse('2026-08-05T10:00:00.000Z'),
      Date.parse('2026-08-05T13:00:00.000Z'),
    ])
    expect(axis.ticks).toEqual([
      Date.parse('2026-08-05T10:00:00.000Z'),
      Date.parse('2026-08-05T11:00:00.000Z'),
      Date.parse('2026-08-05T12:00:00.000Z'),
      Date.parse('2026-08-05T13:00:00.000Z'),
    ])
  })
})
