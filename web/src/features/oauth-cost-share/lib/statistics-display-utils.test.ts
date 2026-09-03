import { describe, expect, it } from 'vitest'
import {
  normalizeAnalyticsBarData,
  normalizeAnalyticsSankeyData,
  normalizeStatisticsLabel,
} from './statistics-display-utils'

describe('statistics display names', () => {
  it.each(['北海自用', '北海（本体）', '北海员工用', '北海（员工用）', '北海新员工密钥'])('maps %s to 北海', (name) => {
    expect(normalizeStatisticsLabel(name)).toBe('北海')
  })

  it('merges North Sea aliases in bar chart data', () => {
    const got = normalizeAnalyticsBarData({
      bucket_unit: 'day',
      series: [
        { model_key: 'gpt-5', model_label: 'gpt-5', api_key_id: 'key-self', api_key_label: '北海自用', is_other: false },
        { model_key: 'gpt-5', model_label: 'gpt-5', api_key_id: 'key-staff', api_key_label: '北海（员工用）', is_other: false },
      ],
      points: [{
        bucket_start: '2026-09-01T00:00:00Z',
        bucket_end: '2026-09-02T00:00:00Z',
        groups: [{
          model_key: 'gpt-5',
          model_label: 'gpt-5',
          total_cost: 3,
          segments: [
            { api_key_id: 'key-self', api_key_label: '北海自用', cost: 1, is_other: false },
            { api_key_id: 'key-staff', api_key_label: '北海（员工用）', cost: 2, is_other: false },
          ],
        }],
      }],
    })

    expect(got.series).toHaveLength(1)
    expect(got.series[0]?.api_key_label).toBe('北海')
    expect(got.points[0]?.groups[0]?.segments).toEqual([
      { api_key_id: 'statistics:beihai', api_key_label: '北海', cost: 3, is_other: false },
    ])
  })

  it('normalizes any matching key prefix before merging bar data', () => {
    const got = normalizeAnalyticsBarData({
      bucket_unit: 'day',
      series: [
        { model_key: 'gpt-5', model_label: 'gpt-5', api_key_id: 'wilson-self', api_key_label: 'Wilson（本体）', is_other: false },
        { model_key: 'gpt-5', model_label: 'gpt-5', api_key_id: 'wilson-staff', api_key_label: 'WIlson（员工用）', is_other: false },
      ],
      points: [{
        bucket_start: '2026-09-01T00:00:00Z',
        bucket_end: '2026-09-02T00:00:00Z',
        groups: [{
          model_key: 'gpt-5',
          model_label: 'gpt-5',
          total_cost: 3,
          segments: [
            { api_key_id: 'wilson-self', api_key_label: 'Wilson（本体）', cost: 1, is_other: false },
            { api_key_id: 'wilson-staff', api_key_label: 'WIlson（员工用）', cost: 2, is_other: false },
          ],
        }],
      }],
    })

    expect(got.series).toHaveLength(1)
    expect(got.series[0]?.api_key_label).toBe('Wilson')
    expect(got.points[0]?.groups[0]?.segments).toEqual([
      { api_key_id: 'statistics:wilson', api_key_label: 'Wilson', cost: 3, is_other: false },
    ])
  })

  it('merges North Sea aliases in Sankey data', () => {
    const got = normalizeAnalyticsSankeyData({
      nodes: [
        { id: 'model:gpt-5', label: 'gpt-5', type: 'model', is_other: false },
        { id: 'key-self', label: '北海自用', type: 'api_key', is_other: false },
        { id: 'key-staff', label: '北海（员工用）', type: 'api_key', is_other: false },
      ],
      links: [
        { source: 'model:gpt-5', target: 'key-self', value: 10 },
        { source: 'model:gpt-5', target: 'key-staff', value: 20 },
      ],
      total_tokens: 30,
    })

    expect(got.nodes.filter((node) => node.type === 'api_key')).toEqual([
      { id: 'statistics:beihai', label: '北海', type: 'api_key', is_other: false },
    ])
    expect(got.links).toEqual([
      { source: 'model:gpt-5', target: 'statistics:beihai', value: 30 },
    ])
  })
})
