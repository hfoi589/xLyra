import { useMemo, useState, type ReactNode } from 'react'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, LoaderCircle, RotateCcw, Save } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useLocation } from 'react-router-dom'
import { ErrorState } from '@/components/common/error-state'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { FormField } from '@/components/ui/form-field'
import { Input } from '@/components/ui/input'
import { toast } from '@/lib/toast'
import {
  getOAuthCostShare,
  getOAuthCostShareConfig,
  oauthCostShareQueryKeys,
  updateOAuthCostShareConfig,
} from '../api/oauth-cost-share'
import { OAuthCostShareChart } from './oauth-cost-share-chart'
import {
	costShareConfigToDraft,
	costShareQueryFromSearch,
	costShareWarning,
	draftToCostShareConfig,
} from '../lib/oauth-cost-share-utils'
import type { OAuthCostShareConfig, OAuthCostShareConfigDraft } from '../lib/types'

const EMPTY_PLAN = { single_quota: 0, reset_count: 0, account_fee: 0 }
const DEFAULT_CONFIG: OAuthCostShareConfig = {
  plus: EMPTY_PLAN,
  pro_lite: EMPTY_PLAN,
  pro: EMPTY_PLAN,
}

const PLAN_KEYS = ['plus', 'pro_lite', 'pro'] as const
type PlanKey = typeof PLAN_KEYS[number]
type DraftField = keyof OAuthCostShareConfigDraft[PlanKey]

export function OAuthCostSharePanel() {
  const { t } = useTranslation('oauth-cost-share')
  const location = useLocation()
  const queryClient = useQueryClient()
  const query = useMemo(() => costShareQueryFromSearch(location.search), [location.search])
  const [draft, setDraft] = useState<OAuthCostShareConfigDraft | null>(null)

  const configQuery = useQuery({
    queryKey: oauthCostShareQueryKeys.config(),
    queryFn: getOAuthCostShareConfig,
    staleTime: 60_000,
  })
  const costShareQuery = useQuery({
    queryKey: query ? oauthCostShareQueryKeys.detail(query) : [...oauthCostShareQueryKeys.all, 'inactive'],
    queryFn: () => getOAuthCostShare(query as NonNullable<typeof query>),
    enabled: Boolean(query),
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  })

  const currentDraft = draft ?? costShareConfigToDraft(configQuery.data ?? DEFAULT_CONFIG)

  const saveMutation = useMutation({
    mutationFn: () => updateOAuthCostShareConfig(draftToCostShareConfig(currentDraft)),
    onSuccess: async (config) => {
      setDraft(costShareConfigToDraft(config))
      await queryClient.invalidateQueries({ queryKey: oauthCostShareQueryKeys.config() })
      await queryClient.invalidateQueries({ queryKey: oauthCostShareQueryKeys.all })
      toast.success(t('config.saved'))
    },
    onError: (error) => toast.error(t('config.saveFailed'), { description: error.message }),
  })

  function patchPlan(plan: PlanKey, field: DraftField, value: string) {
    setDraft((current) => ({
      ...(current ?? costShareConfigToDraft(configQuery.data ?? DEFAULT_CONFIG)),
      [plan]: { ...(current ?? costShareConfigToDraft(configQuery.data ?? DEFAULT_CONFIG))[plan], [field]: value },
    }))
  }

  function restoreDraft() {
    setDraft(costShareConfigToDraft(configQuery.data ?? DEFAULT_CONFIG))
  }

	const data = costShareQuery.data?.data
	const speedDengWarning = costShareWarning(costShareQuery.data)
	const hasOneSite = Boolean(query)
  const unavailableKey = data?.unsupported_reason ? unsupportedReasonKey(data.unsupported_reason) : undefined

  return (
    <div className="space-y-5">
      <Card className="overflow-visible">
        <CardContent className="space-y-4 pt-6">
          <div className="space-y-1">
            <h3 className="text-base font-semibold text-foreground">{t('config.title')}</h3>
            <p className="text-xs leading-5 text-muted-soft">{t('config.description')}</p>
          </div>
          <div className="grid gap-4 md:grid-cols-3">
            {PLAN_KEYS.map((plan) => (
              <div key={plan} className="space-y-3 rounded-xl border border-[hsl(var(--glass-border))] bg-[hsl(var(--surface-panel))] p-4">
                <h4 className="text-sm font-semibold text-foreground">{t(`plans.${plan}`)}</h4>
                <FormField label={t('config.singleQuota')}>
                  <Input
                    type="text"
                    inputMode="decimal"
                    value={currentDraft[plan].single_quota}
                    onChange={(event) => patchPlan(plan, 'single_quota', event.target.value)}
                  />
                </FormField>
                <FormField label={t('config.resetCount')}>
                  <Input
                    type="text"
                    inputMode="numeric"
                    value={currentDraft[plan].reset_count}
                    onChange={(event) => patchPlan(plan, 'reset_count', event.target.value)}
                  />
                </FormField>
                <FormField label={t('config.accountFee')}>
                  <Input
                    type="text"
                    inputMode="decimal"
                    value={currentDraft[plan].account_fee}
                    onChange={(event) => patchPlan(plan, 'account_fee', event.target.value)}
                  />
                </FormField>
              </div>
            ))}
          </div>
          <div className="flex flex-wrap items-center justify-end gap-2">
            <Button type="button" variant="outline" onClick={restoreDraft} disabled={saveMutation.isPending || configQuery.isLoading}>
              <RotateCcw className="h-4 w-4" />{t('config.restore')}
            </Button>
            <Button type="button" onClick={() => saveMutation.mutate()} disabled={saveMutation.isPending || configQuery.isLoading}>
              {saveMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
              {t('config.save')}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card className="overflow-hidden">
        <CardContent className="space-y-4 pt-6">
          <div className="flex items-start justify-between gap-4">
            <div className="space-y-1">
              <h3 className="text-base font-semibold text-foreground">{t('chart.title')}</h3>
              <p className="text-xs leading-5 text-muted-soft">{t('chart.description')}</p>
            </div>
            {costShareQuery.isFetching ? <LoaderCircle className="mt-0.5 h-4 w-4 shrink-0 animate-spin text-primary" aria-label={t('chart.loading')} /> : null}
          </div>
          {speedDengWarning ? <p className="text-xs text-amber-200">{t('chart.speedDengWarning')}: {speedDengWarning}</p> : null}

          {!hasOneSite ? (
            <CostShareStatus>{t('states.singleSite')}</CostShareStatus>
          ) : costShareQuery.isLoading ? (
            <CostShareStatus><LoaderCircle className="mr-2 h-4 w-4 animate-spin" />{t('chart.loading')}</CostShareStatus>
          ) : costShareQuery.error ? (
            <ErrorState title={t('states.loadFailed')} description={(costShareQuery.error as Error).message} />
          ) : data && !data.supported ? (
            <CostShareStatus>
              <AlertTriangle className="mr-2 h-4 w-4 shrink-0 text-amber-300" />
              {t(`states.${unavailableKey ?? 'unsupported'}`)}
            </CostShareStatus>
          ) : data && !data.items.length ? (
            <CostShareStatus>{t('states.empty')}</CostShareStatus>
          ) : data ? (
            <>
              {data.over_quota ? <p className="text-xs text-amber-300">{t('chart.overQuota')}</p> : null}
              <OAuthCostShareChart data={data} />
              <div className="border-t border-[hsl(var(--glass-divider))] pt-3 text-xs text-muted-soft">
                {data.site_label} · {t(`plans.${planKey(data.plan_type)}`)} · {t('chart.totalUsage')}: {formatCurrency(data.total_usage_cost, data.usage_currency ?? 'USD')} · {t('chart.totalQuota')}: {formatCurrency(data.total_quota, data.usage_currency ?? 'USD')}
              </div>
            </>
          ) : null}
        </CardContent>
      </Card>
    </div>
  )
}

function CostShareStatus({ children }: { children: ReactNode }) {
  return <div className="flex min-h-[220px] items-center justify-center rounded-xl border border-dashed border-[hsl(var(--glass-border))] px-6 text-center text-sm text-muted-soft">{children}</div>
}

function unsupportedReasonKey(reason: string) {
  switch (reason) {
    case 'not_oauth': return 'notOAuth'
    case 'unsupported_plan': return 'unsupportedPlan'
    case 'quota_not_configured': return 'quotaNotConfigured'
    case 'fee_not_configured': return 'feeNotConfigured'
    default: return 'unsupported'
  }
}

function planKey(value: string): PlanKey {
  if (value === 'pro lite') return 'pro_lite'
  if (value === 'pro') return 'pro'
  return 'plus'
}

function formatCurrency(value: number, currency: string) {
  if (currency.toUpperCase() === 'USD') return `$${value.toFixed(2)}`
  return `${value.toFixed(2)} ${currency.toUpperCase()}`
}
