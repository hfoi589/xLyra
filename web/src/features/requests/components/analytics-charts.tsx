import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Sankey,
  Scatter,
  ScatterChart,
  Tooltip,
  XAxis,
  YAxis,
  type TooltipContentProps,
  type SankeyNodeProps,
} from 'recharts'
import { dashboardChartColors, dashboardTooltipItemStyle, dashboardTooltipLabelStyle, dashboardTooltipStyle } from '@/features/dashboard/components/chart-style'
import {
  buildBarChartRows,
  buildSankeyChartData,
  buildScatterChartPoints,
  buildScatterTimeAxis,
  type AnalyticsBarData,
  type AnalyticsScatterPoint,
  type AnalyticsSankeyData,
} from '@/features/requests/lib/analytics-utils'

function formatCost(value: number, currency: string) {
  return `${currency} ${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 })}`
}

function formatTokens(value: number) {
  return value.toLocaleString()
}

function formatBucketLabel(value: string | number, unit: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  if (unit === 'hour' || unit === '15min') {
    return date.toLocaleString(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
  }
  if (unit === 'week') {
    return date.toLocaleDateString(undefined, { year: 'numeric', month: '2-digit', day: '2-digit' })
  }
  return date.toLocaleDateString(undefined, { month: '2-digit', day: '2-digit' })
}

export function AnalyticsBarChart({ data, currency }: { data: AnalyticsBarData; currency: string }) {
  const { rows, series } = useMemo(() => buildBarChartRows(data), [data])
  const modelLabels = useMemo(
    () => new Map(series.map((item) => [item.model_key, item.model_label])),
    [series],
  )
  const animation = rows.length < 24

  return (
    <div className="h-[360px] w-full" role="img" aria-label="Request cost by model and downstream API key">
      <ResponsiveContainer width="100%" height="100%">
        <BarChart accessibilityLayer={false} data={rows} margin={{ top: 8, right: 8, bottom: 8, left: 8 }}>
          <CartesianGrid stroke="hsl(var(--glass-border))" vertical={false} />
          <XAxis
            dataKey="model_key"
            tickLine={false}
            axisLine={false}
            interval={0}
            minTickGap={12}
            tickFormatter={(value) => modelLabels.get(String(value)) ?? String(value)}
            tick={{ fill: 'hsl(var(--text-muted-soft))', fontSize: 11 }}
          />
          <YAxis
            tickLine={false}
            axisLine={false}
            width={70}
            tickFormatter={(value) => formatCost(Number(value), currency)}
            tick={{ fill: 'hsl(var(--text-muted-soft))', fontSize: 11 }}
          />
          <Tooltip
            cursor={{ fill: 'hsl(var(--surface-subtle))' }}
            formatter={(value, name) => [formatCost(Number(value ?? 0), currency), String(name ?? '')]}
            contentStyle={dashboardTooltipStyle}
            labelStyle={dashboardTooltipLabelStyle}
            itemStyle={dashboardTooltipItemStyle}
            labelFormatter={(value) => modelLabels.get(String(value)) ?? String(value)}
          />
          <Legend wrapperStyle={{ fontSize: 11 }} />
          {series.map((item, index) => (
            <Bar
              key={item.dataKey}
              dataKey={item.dataKey}
              name={`${item.model_label} · ${item.api_key_label}`}
              stackId={item.stackId}
              fill={dashboardChartColors[index % dashboardChartColors.length]}
              isAnimationActive={animation}
              radius={index === series.length - 1 ? [3, 3, 0, 0] : [0, 0, 0, 0]}
            />
          ))}
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}

type ScatterTooltipProps = TooltipContentProps & { currency: string }

function ScatterTooltip({ active, payload, currency }: ScatterTooltipProps) {
  const { t } = useTranslation('requests')
  if (!active || !payload?.length) return null
  const point = payload[0]?.payload as AnalyticsScatterPoint | undefined
  if (!point) return null
  return (
    <div style={dashboardTooltipStyle} className="min-w-56 space-y-1.5 text-xs">
      <p className="border-b border-[hsl(var(--glass-divider))] pb-1.5 text-muted-soft">
        {t('charts.tooltip.bucket')}: {formatBucketLabel(point.bucket_start, '15min')} – {formatBucketLabel(point.bucket_end, '15min')}
      </p>
      <p>{t('charts.tooltip.requestCount')}: {formatTokens(point.request_count)}</p>
      <p>{t('charts.tooltip.tokens')}: {formatTokens(point.total_tokens)}</p>
      <p>{t('charts.tooltip.totalCost')}: {formatCost(point.total_cost, currency)}</p>
    </div>
  )
}

export function AnalyticsScatterChart({
  points,
  currency,
  rangeStart,
  rangeEnd,
}: {
  points: AnalyticsScatterPoint[]
  currency: string
  rangeStart?: string
  rangeEnd?: string
}) {
  const chartPoints = useMemo(
    () => buildScatterChartPoints(points),
    [points],
  )
  const axis = useMemo(
    () => buildScatterTimeAxis(chartPoints, rangeStart, rangeEnd),
    [chartPoints, rangeEnd, rangeStart],
  )
  const animation = chartPoints.length < 500
  return (
    <div className="h-[360px] w-full" role="img" aria-label="Request cost by fifteen-minute time bucket">
      <ResponsiveContainer width="100%" height="100%">
        <ScatterChart accessibilityLayer={false} margin={{ top: 12, right: 18, bottom: 12, left: 8 }}>
          <CartesianGrid stroke="hsl(var(--glass-border))" />
          <XAxis
            type="number"
            dataKey="bucket_timestamp"
            name="Time"
            domain={axis.domain}
            ticks={axis.ticks}
            tickFormatter={(value) => String(formatBucketLabel(Number(value), '15min'))}
            interval={axis.ticks.length > 12 ? 'preserveStartEnd' : 0}
            minTickGap={18}
            tickLine={false}
            axisLine={false}
            tick={{ fill: 'hsl(var(--text-muted-soft))', fontSize: 11 }}
          />
          <YAxis
            type="number"
            dataKey="total_cost"
            name={currency}
            tickFormatter={(value) => formatCost(Number(value), currency)}
            tickLine={false}
            axisLine={false}
            width={74}
            tick={{ fill: 'hsl(var(--text-muted-soft))', fontSize: 11 }}
          />
          <Tooltip content={(props) => <ScatterTooltip {...props} currency={currency} />} />
          <Scatter data={chartPoints} fill="hsl(var(--primary))" fillOpacity={0.62} isAnimationActive={animation} />
        </ScatterChart>
      </ResponsiveContainer>
    </div>
  )
}

type AnalyticsSankeyNodePayload = SankeyNodeProps['payload'] & {
  id?: string
  label?: string
  type?: string
  is_other?: boolean
}

const sankeyNodeColors: Record<string, string> = {
  site: 'hsl(218 74% 58%)',
  model: 'hsl(268 54% 62%)',
  api_key: 'hsl(42 88% 48%)',
  other: 'hsl(190 58% 48%)',
}

function renderSankeyNode({ x, y, width, height, payload }: SankeyNodeProps) {
  const node = payload as AnalyticsSankeyNodePayload
  const type = node.type ?? 'other'
  const color = sankeyNodeColors[type] ?? sankeyNodeColors.other
  const label = node.label ?? node.name
  const isRightmost = type === 'api_key'
  const nodeHeight = Math.max(height, 8)

  return (
    <g>
      <rect
        x={x}
        y={y}
        width={width}
        height={nodeHeight}
        rx={4}
        fill={color}
        fillOpacity={node.is_other ? 0.62 : 0.92}
        stroke="hsl(var(--foreground) / 0.22)"
        strokeWidth={1}
      />
      <text
        x={isRightmost ? x - 8 : x + width + 8}
        y={y + nodeHeight / 2}
        textAnchor={isRightmost ? 'end' : 'start'}
        dominantBaseline="middle"
        fill="hsl(var(--foreground))"
        fontSize={11}
        fontWeight={type === 'model' ? 600 : 500}
      >
        <title>{label}</title>
        {label}
      </text>
    </g>
  )
}

export function AnalyticsSankeyChart({ data }: { data: AnalyticsSankeyData }) {
  const chartData = useMemo(() => buildSankeyChartData(data), [data])
  const { t } = useTranslation('requests')

  return (
    <div className="w-full" role="img" aria-label="Site to model to API key token flow">
      <div className="mb-2 flex flex-wrap items-center gap-x-5 gap-y-2 text-[11px] text-muted-soft">
        {(['site', 'model', 'api_key'] as const).map((type) => (
          <span key={type} className="inline-flex items-center gap-1.5">
            <span className="h-2.5 w-2.5 rounded-sm" style={{ backgroundColor: sankeyNodeColors[type] }} />
            {t(`charts.sankey.legend.${type}`)}
          </span>
        ))}
      </div>
      <div className="h-[380px] w-full">
        <ResponsiveContainer width="100%" height="100%">
          <Sankey
            data={chartData}
            node={renderSankeyNode}
            nodePadding={30}
            nodeWidth={16}
            linkCurvature={0.52}
            iterations={32}
            link={{ stroke: 'hsl(var(--glass-border-strong))', strokeOpacity: 0.46 }}
            margin={{ top: 16, right: 24, bottom: 12, left: 24 }}
          >
            <Tooltip
              formatter={(value) => [formatTokens(Number(value ?? 0)), 'Tokens']}
              contentStyle={dashboardTooltipStyle}
              labelStyle={dashboardTooltipLabelStyle}
              itemStyle={dashboardTooltipItemStyle}
            />
          </Sankey>
        </ResponsiveContainer>
      </div>
    </div>
  )
}
