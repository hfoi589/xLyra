import { useMemo, useState, type ReactNode } from 'react'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { LoaderCircle, RotateCcw } from 'lucide-react'
import { useLocation, useNavigate } from 'react-router-dom'
import { ErrorState } from '@/components/common/error-state'
import { PageHeader } from '@/components/common/page-header'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { DateTimePicker } from '@/components/ui/date-time-picker'
import { MultiSelect, type MultiSelectOption } from '@/components/ui/multi-select'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { getRequestAnalytics, analyticsQueryKeys } from '@/features/requests/api/request-analytics'
import {
  AnalyticsBarChart,
  AnalyticsSankeyChart,
  AnalyticsScatterChart,
} from '@/features/requests/components/analytics-charts'
import {
  analyticsSearchParams,
  normalizeAnalyticsFilters,
  type AnalyticsBarData,
  type AnalyticsFilterState,
  type AnalyticsScatterPoint,
  type AnalyticsSankeyData,
} from '@/features/requests/lib/analytics-utils'
import { listCanonicalModels, modelsQueryKeys } from '@/features/models/api/models'
import { listDownstreamAPIKeys, downstreamAPIKeyQueryKeys } from '@/features/api-keys/api/api-keys'
import { listSites, sitesQueryKeys } from '@/features/sites/api/sites'
import { normalizeAnalyticsBarData, normalizeAnalyticsSankeyData } from '@/features/oauth-cost-share/lib/statistics-display-utils'
import { SpeedDengControl } from '@/features/speed-deng/components/speed-deng-control'

type AnalyticsDraftFilters = {
  createdFrom?: Date
  createdTo?: Date
  modelKeys: string[]
  siteIds: string[]
  apiKeyIds: string[]
  currency: string
}

const DEFAULT_ANALYTICS_DAYS = 7

function defaultAnalyticsDraft(now = new Date()): AnalyticsDraftFilters {
  return {
    createdFrom: new Date(now.getTime() - DEFAULT_ANALYTICS_DAYS * 24 * 60 * 60 * 1000),
    createdTo: now,
    modelKeys: [],
    siteIds: [],
    apiKeyIds: [],
    currency: '',
  }
}

function draftFromSearch(search: string): AnalyticsDraftFilters {
  const params = new URLSearchParams(search)
  const initial = defaultAnalyticsDraft()
  const parseDate = (value: string | null) => {
    if (!value) return undefined
    const date = new Date(value)
    return Number.isNaN(date.getTime()) ? undefined : date
  }
  return {
    ...initial,
    createdFrom: parseDate(params.get('created_from')) ?? initial.createdFrom,
    createdTo: parseDate(params.get('created_to')) ?? initial.createdTo,
    modelKeys: params.getAll('model_key'),
    siteIds: params.getAll('site_id'),
    apiKeyIds: params.getAll('api_key_id'),
    currency: params.get('currency')?.trim().toUpperCase() ?? '',
  }
}

function draftToApplied(draft: AnalyticsDraftFilters): AnalyticsFilterState {
  return normalizeAnalyticsFilters({
    createdFrom: draft.createdFrom?.toISOString(),
    createdTo: draft.createdTo?.toISOString(),
    modelKeys: draft.modelKeys,
    siteIds: draft.siteIds,
    apiKeyIds: draft.apiKeyIds,
    currency: draft.currency,
  })
}

function ChartCard({
  title,
  description,
  isLoading,
  isFetching,
  error,
  empty,
  children,
  footer,
}: {
  title: string
  description: string
  isLoading: boolean
  isFetching: boolean
  error: Error | null
  empty: boolean
  children: ReactNode
  footer?: ReactNode
}) {
  const { t } = useTranslation('request-charts')
  return (
    <Card className="overflow-hidden">
      <CardContent className="space-y-4 pt-6">
        <div className="flex items-start justify-between gap-4">
          <div className="space-y-1">
            <h3 className="text-base font-semibold text-foreground">{title}</h3>
            <p className="text-xs leading-5 text-muted-soft">{description}</p>
          </div>
          {isFetching ? <LoaderCircle className="mt-0.5 h-4 w-4 shrink-0 animate-spin text-primary" aria-label={t('charts.loading')} /> : null}
        </div>
        {isLoading ? (
          <div className="flex h-[360px] items-center justify-center rounded-xl border border-dashed border-[hsl(var(--glass-border))] text-sm text-muted-soft">
            <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />{t('charts.loading')}
          </div>
        ) : error && empty ? (
          <ErrorState title={t('charts.loadFailed')} description={error.message} />
        ) : empty ? (
          <div className="flex h-[360px] items-center justify-center rounded-xl border border-dashed border-[hsl(var(--glass-border))] text-sm text-muted-soft">
            {t('charts.empty')}
          </div>
        ) : (
          children
        )}
        {error && !empty ? <p className="text-xs text-red-300">{t('charts.refreshFailed')}: {error.message}</p> : null}
        {footer ? <div className="border-t border-[hsl(var(--glass-divider))] pt-3 text-xs text-muted-soft">{footer}</div> : null}
      </CardContent>
    </Card>
  )
}

export function RequestsAnalyticsWorkspace() {
  const { t } = useTranslation('requests')
  const { t: chartT } = useTranslation('request-charts')
  const location = useLocation()
  const navigate = useNavigate()
  const [draft, setDraft] = useState<AnalyticsDraftFilters>(() => draftFromSearch(location.search))
  const [appliedFilters, setAppliedFilters] = useState<AnalyticsFilterState>(() => draftToApplied(draft))

  const modelsQuery = useQuery({
    queryKey: modelsQueryKeys.canonical(),
    queryFn: listCanonicalModels,
    staleTime: 60_000,
  })
  const sitesQuery = useQuery({
    queryKey: sitesQueryKeys.list('all', 'with_requests'),
    queryFn: () => listSites({ oauth: 'all', deleted: 'with_requests' }),
    staleTime: 60_000,
  })
  const apiKeysQuery = useQuery({
    queryKey: downstreamAPIKeyQueryKeys.list(),
    queryFn: listDownstreamAPIKeys,
    staleTime: 60_000,
  })
  const barQuery = useQuery({
    queryKey: analyticsQueryKeys.view('bar', appliedFilters),
    queryFn: () => getRequestAnalytics('bar', appliedFilters),
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  })
  const scatterQuery = useQuery({
    queryKey: analyticsQueryKeys.view('scatter', appliedFilters),
    queryFn: () => getRequestAnalytics('scatter', appliedFilters),
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  })
  const sankeyQuery = useQuery({
    queryKey: analyticsQueryKeys.view('sankey', appliedFilters),
    queryFn: () => getRequestAnalytics('sankey', appliedFilters),
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  })

  const availableCurrencies = useMemo(() => {
    const values = [
      ...(barQuery.data?.meta.available_currencies ?? []),
      ...(scatterQuery.data?.meta.available_currencies ?? []),
      ...(sankeyQuery.data?.meta.available_currencies ?? []),
    ]
    return [...new Set(values.filter(Boolean))].sort()
  }, [barQuery.data?.meta.available_currencies, scatterQuery.data?.meta.available_currencies, sankeyQuery.data?.meta.available_currencies])

  const modelOptions: MultiSelectOption[] = (modelsQuery.data?.items ?? []).map((model) => ({
    value: model.model_key,
    label: model.display_name || model.model_key,
    description: model.model_key,
  }))
  const siteOptions: MultiSelectOption[] = (sitesQuery.data?.items ?? []).map((site) => ({ value: site.id, label: site.name, description: site.slug }))
  const apiKeyOptions: MultiSelectOption[] = (apiKeysQuery.data?.items ?? []).map((apiKey) => ({ value: apiKey.id, label: apiKey.name }))
  const barData = useMemo(() => {
    const raw = barQuery.data?.data as AnalyticsBarData | undefined
    return raw ? normalizeAnalyticsBarData(raw) : undefined
  }, [barQuery.data?.data])
  const scatterData = scatterQuery.data?.data as { points: AnalyticsScatterPoint[] } | undefined
  const sankeyData = useMemo(() => {
    const raw = sankeyQuery.data?.data as AnalyticsSankeyData | undefined
    return raw ? normalizeAnalyticsSankeyData(raw) : undefined
  }, [sankeyQuery.data?.data])
  const isFetching = barQuery.isFetching || scatterQuery.isFetching || sankeyQuery.isFetching

  function patchDraft(patch: Partial<AnalyticsDraftFilters>) {
    setDraft((current) => ({ ...current, ...patch }))
  }

  function applyFilters(nextDraft = draft) {
    const next = draftToApplied(nextDraft)
    setAppliedFilters(next)
    const params = analyticsSearchParams('bar', next)
    navigate({ pathname: location.pathname, search: `?${params.toString()}` }, { replace: true })
  }

  function resetFilters() {
    const nextDraft = defaultAnalyticsDraft()
    setDraft(nextDraft)
    applyFilters(nextDraft)
  }

  return (
    <div className="flex min-h-full flex-col gap-6">
      <PageHeader
        eyebrow={t('page.eyebrow')}
        title={chartT('charts.title')}
        description={chartT('charts.description')}
        actions={<SpeedDengControl />}
      />

      <Card className="relative z-[130] overflow-visible">
        <CardContent className="space-y-4 pt-6">
          <div className="flex flex-wrap items-center gap-3">
            <DateTimePicker
              value={draft.createdFrom}
              onValueChange={(value) => patchDraft({ createdFrom: value })}
              placeholder={t('filters.dateFrom')}
              minuteStep={1}
              disableFutureDates
              className="w-64"
              triggerClassName="h-10"
            />
            <DateTimePicker
              value={draft.createdTo}
              onValueChange={(value) => patchDraft({ createdTo: value })}
              placeholder={t('filters.dateTo')}
              minuteStep={1}
              disableFutureDates
              className="w-64"
              triggerClassName="h-10"
            />
            <MultiSelect
              value={draft.modelKeys}
              options={modelOptions}
              onChange={(modelKeys) => patchDraft({ modelKeys })}
              placeholder={chartT('charts.filters.models')}
              searchPlaceholder={chartT('charts.filters.searchModels')}
              emptyText={chartT('charts.filters.noModels')}
              selectedText={chartT('charts.filters.selected')}
              triggerVariant="filter"
              triggerLabel={chartT('charts.filters.models')}
              dropdownWidthMode="content"
            />
            <MultiSelect
              value={draft.siteIds}
              options={siteOptions}
              onChange={(siteIds) => patchDraft({ siteIds })}
              placeholder={chartT('charts.filters.sites')}
              searchPlaceholder={chartT('charts.filters.searchSites')}
              emptyText={chartT('charts.filters.noSites')}
              selectedText={chartT('charts.filters.selected')}
              triggerVariant="filter"
              triggerLabel={chartT('charts.filters.sites')}
              dropdownWidthMode="content"
            />
            <MultiSelect
              value={draft.apiKeyIds}
              options={apiKeyOptions}
              onChange={(apiKeyIds) => patchDraft({ apiKeyIds })}
              placeholder={chartT('charts.filters.apiKeys')}
              searchPlaceholder={chartT('charts.filters.searchApiKeys')}
              emptyText={chartT('charts.filters.noApiKeys')}
              selectedText={chartT('charts.filters.selected')}
              triggerVariant="filter"
              triggerLabel={chartT('charts.filters.apiKeys')}
              dropdownWidthMode="content"
            />
            {availableCurrencies.length > 1 ? (
              <Select value={draft.currency || availableCurrencies[0]} onValueChange={(currency) => patchDraft({ currency })}>
                <SelectTrigger variant="filter" filterLabel={chartT('charts.filters.currency')} active={Boolean(draft.currency)}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent searchable={false} widthMode="content">
                  {availableCurrencies.map((currency) => <SelectItem key={currency} value={currency}>{currency}</SelectItem>)}
                </SelectContent>
              </Select>
            ) : null}
          </div>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <p className="text-xs text-muted-soft">{chartT('charts.filters.applyHint')}</p>
            <div className="flex items-center gap-2">
              <Button type="button" variant="outline" onClick={resetFilters} disabled={isFetching}>
                <RotateCcw className="h-4 w-4" />{chartT('charts.filters.reset')}
              </Button>
              <Button type="button" onClick={() => applyFilters()} disabled={isFetching}>
                {isFetching ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
                {chartT('charts.filters.apply')}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-5">
        <ChartCard
          title={chartT('charts.bar.title')}
          description={chartT('charts.bar.description')}
          isLoading={barQuery.isLoading}
          isFetching={barQuery.isFetching}
          error={barQuery.error as Error | null}
          empty={!barData?.points.length}
          footer={barQuery.data ? `${chartT('charts.totalRequests')}: ${barQuery.data.meta.total_requests.toLocaleString()} · ${chartT('charts.currency')}: ${barQuery.data.meta.currency}` : undefined}
        >
          {barData ? <AnalyticsBarChart data={barData} currency={barQuery.data?.meta.currency ?? 'USD'} /> : null}
        </ChartCard>
        <ChartCard
          title={chartT('charts.scatter.title')}
          description={chartT('charts.scatter.description')}
          isLoading={scatterQuery.isLoading}
          isFetching={scatterQuery.isFetching}
          error={scatterQuery.error as Error | null}
          empty={!scatterData?.points.length && !scatterQuery.data?.meta.too_many_points}
          footer={scatterQuery.data?.meta.too_many_points ? chartT('charts.scatter.tooMany') : scatterQuery.data ? `${chartT('charts.totalRequests')}: ${scatterQuery.data.meta.total_requests.toLocaleString()}` : undefined}
        >
          {scatterQuery.data?.meta.too_many_points ? (
            <div className="flex h-[360px] items-center justify-center rounded-xl border border-dashed border-amber-400/30 bg-amber-400/5 px-6 text-center text-sm text-amber-100">
              {chartT('charts.scatter.tooMany')}
            </div>
          ) : scatterData ? (
            <AnalyticsScatterChart
              points={scatterData.points}
              currency={scatterQuery.data?.meta.currency ?? 'USD'}
              rangeStart={appliedFilters.createdFrom}
              rangeEnd={appliedFilters.createdTo}
            />
          ) : null}
        </ChartCard>
        <ChartCard
          title={chartT('charts.sankey.title')}
          description={chartT('charts.sankey.description')}
          isLoading={sankeyQuery.isLoading}
          isFetching={sankeyQuery.isFetching}
          error={sankeyQuery.error as Error | null}
          empty={!sankeyData?.links.length}
          footer={sankeyData ? `${chartT('charts.totalTokens')}: ${sankeyData.total_tokens.toLocaleString()}` : undefined}
        >
          {sankeyData ? <AnalyticsSankeyChart data={sankeyData} /> : null}
        </ChartCard>
      </div>
    </div>
  )
}
