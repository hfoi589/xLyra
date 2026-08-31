export type AnalyticsView = 'bar' | 'scatter' | 'sankey'

export type AnalyticsFilterState = {
  createdFrom?: string
  createdTo?: string
  modelKeys: string[]
  siteIds: string[]
  apiKeyIds: string[]
  currency: string
}

export type AnalyticsBarSeries = {
  model_key: string
  model_label: string
  api_key_id: string
  api_key_label: string
  is_other: boolean
}

export type AnalyticsBarData = {
  bucket_unit: string
  series: AnalyticsBarSeries[]
  points: Array<{
    bucket_start: string
    bucket_end: string
    groups: Array<{
      model_key: string
      model_label: string
      total_cost: number
      segments: Array<{
        api_key_id: string
        api_key_label: string
        cost: number
        is_other: boolean
      }>
    }>
  }>
}

export type AnalyticsScatterPoint = {
  bucket_start: string
  bucket_end: string
  request_count: number
  total_tokens: number
  total_cost: number
  currency: string
}

export type AnalyticsSankeyData = {
  nodes: Array<{ id: string; label: string; type: string; is_other: boolean }>
  links: Array<{ source: string; target: string; value: number }>
  total_tokens: number
}

export type AnalyticsResponse<T> = {
  meta: {
    view: AnalyticsView
    timezone: string
    range_start: string
    range_end: string
    bucket_unit: string
    currency: string
    available_currencies: string[]
    truncated: boolean
    too_many_points: boolean
    total_requests: number
    returned_points: number
    missing_cost_requests: number
    suggested_action?: string
  }
  data: T
}

export type RechartsBarSeries = AnalyticsBarSeries & {
  dataKey: string
  stackId: string
}

export type RechartsBarRow = {
  model_key: string
  model_label: string
  [key: string]: string | number
}

export type RechartsScatterPoint = AnalyticsScatterPoint & {
  bucket_timestamp: number
}

export type RechartsScatterAxis = {
  domain: [number, number]
  ticks: number[]
}

export type RechartsSankeyData = {
  nodes: Array<{ name: string; label: string; id: string; type: string; is_other: boolean }>
  links: Array<{ source: number; target: number; value: number }>
}

export function normalizeAnalyticsFilters(filters: AnalyticsFilterState): AnalyticsFilterState {
  return {
    createdFrom: filters.createdFrom || undefined,
    createdTo: filters.createdTo || undefined,
    modelKeys: [...new Set(filters.modelKeys.map((value) => value.trim()).filter(Boolean))].sort(),
    siteIds: [...new Set(filters.siteIds.filter(Boolean))].sort(),
    apiKeyIds: [...new Set(filters.apiKeyIds.filter(Boolean))].sort(),
    currency: filters.currency.trim().toUpperCase(),
  }
}

export function analyticsSearchParams(view: AnalyticsView, filters: AnalyticsFilterState) {
  const normalized = normalizeAnalyticsFilters(filters)
  const params = new URLSearchParams()
  params.set('view', view)
  if (normalized.createdFrom) params.set('created_from', normalized.createdFrom)
  if (normalized.createdTo) params.set('created_to', normalized.createdTo)
  normalized.modelKeys.forEach((value) => params.append('model_key', value))
  normalized.siteIds.forEach((value) => params.append('site_id', value))
  normalized.apiKeyIds.forEach((value) => params.append('api_key_id', value))
  if (normalized.currency) params.set('currency', normalized.currency)
  return params
}

export function analyticsSeriesDataKey(modelKey: string, apiKeyId: string) {
  return `series_${encodeURIComponent(modelKey)}_${encodeURIComponent(apiKeyId)}`
}

export function analyticsSeriesStackId(modelKey: string) {
  return `model_${encodeURIComponent(modelKey)}`
}

export function buildBarChartRows(data: AnalyticsBarData) {
  const series: RechartsBarSeries[] = data.series.map((item) => ({
    ...item,
    dataKey: analyticsSeriesDataKey(item.model_key, item.api_key_id),
    stackId: analyticsSeriesStackId(item.model_key),
  }))
  const seriesByPair = new Map(series.map((item) => [`${item.model_key}\u0000${item.api_key_id}`, item]))
  const modelOrder: string[] = []
  const rowsByModel = new Map<string, RechartsBarRow>()

  for (const item of series) {
    if (rowsByModel.has(item.model_key)) continue
    modelOrder.push(item.model_key)
    const row: RechartsBarRow = {
      model_key: item.model_key,
      model_label: item.model_label,
    }
    for (const modelSeries of series.filter((candidate) => candidate.model_key === item.model_key)) {
      row[modelSeries.dataKey] = 0
    }
    rowsByModel.set(item.model_key, row)
  }

  for (const point of data.points) {
    for (const group of point.groups) {
      const row = rowsByModel.get(group.model_key)
      if (!row) continue
      for (const segment of group.segments) {
        const item = seriesByPair.get(`${group.model_key}\u0000${segment.api_key_id}`)
        if (!item) continue
        row[item.dataKey] = Number(row[item.dataKey] ?? 0) + segment.cost
      }
    }
  }

  const rows = modelOrder.flatMap((modelKey) => {
    const row = rowsByModel.get(modelKey)
    return row ? [row] : []
  })
  return { rows, series }
}

export function buildScatterChartPoints(points: AnalyticsScatterPoint[]): RechartsScatterPoint[] {
  return points
    .map((point) => ({ ...point, bucket_timestamp: Date.parse(point.bucket_start) }))
    .filter((point) => Number.isFinite(point.bucket_timestamp))
}

export function buildScatterTimeAxis(
  points: RechartsScatterPoint[],
  rangeStart?: string,
  rangeEnd?: string,
): RechartsScatterAxis {
  const pointTimestamps = points.map((point) => point.bucket_timestamp).filter(Number.isFinite)
  const pointStart = pointTimestamps.length ? Math.min(...pointTimestamps) : 0
  const pointEnd = pointTimestamps.length ? Math.max(...pointTimestamps) + 15 * 60 * 1000 : pointStart + 60 * 60 * 1000
  const parsedStart = rangeStart ? Date.parse(rangeStart) : Number.NaN
  const parsedEnd = rangeEnd ? Date.parse(rangeEnd) : Number.NaN
  const start = Number.isFinite(parsedStart) ? parsedStart : pointStart
  const requestedEnd = Number.isFinite(parsedEnd) ? parsedEnd : pointEnd
  const end = requestedEnd > start ? requestedEnd : start + 60 * 60 * 1000
  const hour = 60 * 60 * 1000
  const durationHours = (end - start) / hour
  const tickStep = Math.max(hour, Math.ceil(durationHours / 24) * hour)
  const ticks = [start]
  for (let timestamp = start + tickStep; timestamp < end; timestamp += tickStep) {
    ticks.push(timestamp)
  }
  if (ticks[ticks.length - 1] !== end) ticks.push(end)
  return { domain: [start, end], ticks }
}

export function buildSankeyChartData(data: AnalyticsSankeyData): RechartsSankeyData {
  const nodes = data.nodes.map((node) => ({
    name: node.label,
    label: node.label,
    id: node.id,
    type: node.type,
    is_other: node.is_other,
  }))
  const nodeIndexes = new Map(nodes.map((node, index) => [node.id, index]))
  const links = data.links.flatMap((link) => {
    const source = nodeIndexes.get(link.source)
    const target = nodeIndexes.get(link.target)
    if (source === undefined || target === undefined) return []
    return [{ source, target, value: link.value }]
  })
  return { nodes, links }
}
