import type {
  AnalyticsBarData,
  AnalyticsSankeyData,
} from '@/features/requests/lib/analytics-utils'

const BEIHAI_STATISTICS_KEY = 'statistics:beihai'

export function normalizeStatisticsLabel(value: string) {
  const name = value.trim()
  const prefix = name.split(/[（(]/, 1)[0]?.trim() ?? name
  if (prefix.startsWith('北海')) return '北海'
  for (const suffix of ['员工用', '自用']) {
    if (prefix.endsWith(suffix) && prefix.length > suffix.length) {
      return prefix.slice(0, -suffix.length).trim()
    }
  }
  return prefix
}

function statisticsAPIKeyId(id: string, label: string) {
  const normalized = normalizeStatisticsLabel(label)
  if (!normalized) return id
  return normalized === '北海' ? BEIHAI_STATISTICS_KEY : `statistics:${normalized.toLowerCase()}`
}

export function normalizeAnalyticsBarData(data: AnalyticsBarData): AnalyticsBarData {
  const seriesByKey = new Map<string, AnalyticsBarData['series'][number]>()
  for (const item of data.series) {
    const apiKeyId = statisticsAPIKeyId(item.api_key_id, item.api_key_label)
    const key = `${item.model_key}\u0000${apiKeyId}`
    const normalized = {
      ...item,
      api_key_id: apiKeyId,
      api_key_label: normalizeStatisticsLabel(item.api_key_label),
    }
    const existing = seriesByKey.get(key)
    seriesByKey.set(key, existing ? { ...existing, is_other: existing.is_other || normalized.is_other } : normalized)
  }

  return {
    ...data,
    series: [...seriesByKey.values()],
    points: data.points.map((point) => ({
      ...point,
      groups: point.groups.map((group) => {
        const segmentsByKey = new Map<string, typeof group.segments[number]>()
        for (const segment of group.segments) {
          const apiKeyId = statisticsAPIKeyId(segment.api_key_id, segment.api_key_label)
          const key = apiKeyId
          const normalized = {
            ...segment,
            api_key_id: apiKeyId,
            api_key_label: normalizeStatisticsLabel(segment.api_key_label),
          }
          const existing = segmentsByKey.get(key)
          segmentsByKey.set(
            key,
            existing
              ? { ...existing, cost: existing.cost + normalized.cost, is_other: existing.is_other || normalized.is_other }
              : normalized,
          )
        }
        return { ...group, segments: [...segmentsByKey.values()] }
      }),
    })),
  }
}

export function normalizeAnalyticsSankeyData(data: AnalyticsSankeyData): AnalyticsSankeyData {
  const nodeIds = new Map<string, string>()
  const nodesByID = new Map<string, AnalyticsSankeyData['nodes'][number]>()

  for (const node of data.nodes) {
    const label = node.type === 'api_key' ? normalizeStatisticsLabel(node.label) : node.label
    const id = node.type === 'api_key' ? statisticsAPIKeyId(node.id, node.label) : node.id
    nodeIds.set(node.id, id)
    const existing = nodesByID.get(id)
    nodesByID.set(id, existing ? { ...existing, is_other: existing.is_other || node.is_other } : { ...node, id, label })
  }

  const linksByKey = new Map<string, AnalyticsSankeyData['links'][number]>()
  for (const link of data.links) {
    const source = nodeIds.get(link.source) ?? link.source
    const target = nodeIds.get(link.target) ?? link.target
    const key = `${source}\u0000${target}`
    const existing = linksByKey.get(key)
    linksByKey.set(key, existing ? { ...existing, value: existing.value + link.value } : { ...link, source, target })
  }

  return { ...data, nodes: [...nodesByID.values()], links: [...linksByKey.values()] }
}
