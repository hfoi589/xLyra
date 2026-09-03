import { apiFetch } from '@/lib/http'
import {
  analyticsSearchParams,
  type AnalyticsBarData,
  type AnalyticsFilterState,
  type AnalyticsResponse,
  type AnalyticsScatterPoint,
  type AnalyticsSankeyData,
  type AnalyticsView,
} from '@/features/requests/lib/analytics-utils'

export const analyticsQueryKeys = {
  all: ['request-analytics'] as const,
  view: (view: AnalyticsView, filters: AnalyticsFilterState) => [
    ...analyticsQueryKeys.all,
    view,
    filters,
  ] as const,
}

export type AnalyticsViewData = {
  bar: AnalyticsResponse<AnalyticsBarData>
  scatter: AnalyticsResponse<{ points: AnalyticsScatterPoint[] }>
  sankey: AnalyticsResponse<AnalyticsSankeyData>
}

export async function getRequestAnalytics<T extends AnalyticsView>(view: T, filters: AnalyticsFilterState) {
  const params = analyticsSearchParams(view, filters)
  return apiFetch<AnalyticsViewData[T]>(`/api/v1/requests/analytics?${params.toString()}`)
}
