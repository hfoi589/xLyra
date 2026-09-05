import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  type TooltipContentProps,
} from 'recharts'
import { dashboardChartColors, dashboardTooltipItemStyle, dashboardTooltipLabelStyle, dashboardTooltipStyle } from '@/features/dashboard/components/chart-style'
import { formatDashboardCurrency, formatPercent } from '@/features/dashboard/lib/dashboard-utils'
import { buildCostSharePieSlices } from '../lib/oauth-cost-share-utils'
import type { OAuthCostShareData, OAuthCostSharePieSlice } from '../lib/types'

type ChartSlice = OAuthCostSharePieSlice & { color: string }

export function OAuthCostShareChart({ data }: { data: OAuthCostShareData }) {
  const { t } = useTranslation('oauth-cost-share')
  const usageCurrency = data.usage_currency ?? 'USD'
  const feeCurrency = data.fee_currency ?? 'CNY'
  const slices = useMemo<ChartSlice[]>(
    () => buildCostSharePieSlices(data, t('chart.unallocated')).map((slice, index) => ({
      ...slice,
      color: slice.is_unallocated ? 'hsl(var(--text-muted-soft))' : dashboardChartColors[index % dashboardChartColors.length],
    })),
    [data, t],
  )
  const total = slices.reduce((sum, slice) => sum + slice.value, 0)

  return (
    <div className="flex min-h-[360px] flex-col items-center gap-6 md:flex-row">
      <div className="relative h-[300px] w-full shrink-0 md:w-[52%]">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart accessibilityLayer={false}>
            <Pie
              data={slices}
              dataKey="value"
              nameKey="name"
              cx="50%"
              cy="50%"
              innerRadius="52%"
              outerRadius="78%"
              paddingAngle={slices.length > 8 ? 1 : 2}
              stroke="hsl(var(--card))"
              strokeWidth={1.5}
              isAnimationActive={slices.length < 30}
            >
              {slices.map((slice) => <Cell key={slice.name} fill={slice.color} />)}
            </Pie>
            <Tooltip
              isAnimationActive={false}
              labelStyle={dashboardTooltipLabelStyle}
              itemStyle={dashboardTooltipItemStyle}
              content={(props) => <CostShareTooltip {...props} usageCurrency={usageCurrency} feeCurrency={feeCurrency} />}
            />
          </PieChart>
        </ResponsiveContainer>
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-xs text-muted-soft">{t('chart.allocated')}</span>
          <span className="mt-1 text-lg font-semibold tabular-nums text-foreground">
            {formatDashboardCurrency(data.allocated_cost, feeCurrency, 2)}
          </span>
          <span className="mt-1 text-[11px] text-muted-soft">
            {formatPercent(data.total_usage_ratio, 1)} {t('chart.ofQuota')}
          </span>
        </div>
      </div>
      <div className="max-h-[300px] min-w-0 flex-1 overflow-y-auto pr-1">
        <div className="space-y-2.5">
          {slices.map((slice) => (
            <div key={slice.name} className="flex items-center gap-2 text-xs">
              <span className="h-2.5 w-2.5 shrink-0 rounded-[3px]" style={{ backgroundColor: slice.color }} />
              <span className="min-w-0 flex-1 truncate text-foreground" title={slice.name}>{slice.name}</span>
              <span className="shrink-0 tabular-nums text-muted-soft">
                {formatDashboardCurrency(slice.value, feeCurrency, 2)} · {formatPercent(slice.usage_share, 1)}
              </span>
            </div>
          ))}
        </div>
        <div className="mt-4 border-t border-[hsl(var(--glass-divider))] pt-3 text-xs text-muted-soft">
          {t('chart.total')}: {formatDashboardCurrency(total, feeCurrency, 2)} · {t('chart.accountFee')}: {formatDashboardCurrency(data.account_fee, feeCurrency, 2)}
        </div>
      </div>
    </div>
  )
}

function CostShareTooltip({ active, payload, usageCurrency, feeCurrency }: TooltipContentProps & { usageCurrency: string; feeCurrency: string }) {
  const { t } = useTranslation('oauth-cost-share')
  if (!active || !payload?.length) return null
  const slice = payload[0]?.payload as ChartSlice | undefined
  if (!slice) return null
  return (
    <div style={dashboardTooltipStyle} className="min-w-[190px] space-y-1.5 text-xs">
      <p className="border-b border-[hsl(var(--glass-divider))] pb-1.5 font-medium text-foreground">{slice.name}</p>
      {!slice.is_unallocated ? <p>{t('tooltip.usageCost')}: {formatDashboardCurrency(slice.usage_cost, usageCurrency, 4)}</p> : null}
      <p>{t('tooltip.usageShare')}: {formatPercent(slice.usage_share, 2)}</p>
      <p>{t('tooltip.allocatedCost')}: {formatDashboardCurrency(slice.allocated_cost, feeCurrency, 4)}</p>
    </div>
  )
}
