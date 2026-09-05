import type {
  OAuthCostShareConfig,
  OAuthCostShareConfigDraft,
  OAuthCostShareData,
  OAuthCostSharePieSlice,
  OAuthCostShareQuery,
  OAuthCostShareResponse,
} from './types'

export function costShareQueryParams(query: OAuthCostShareQuery) {
  const params = new URLSearchParams()
  params.set('site_id', query.siteId)
  if (query.createdFrom) params.set('created_from', query.createdFrom)
  if (query.createdTo) params.set('created_to', query.createdTo)
  return params
}

export function costShareQueryFromSearch(search: string): OAuthCostShareQuery | null {
  const params = new URLSearchParams(search)
  const siteIds = params.getAll('site_id').map((value) => value.trim()).filter(Boolean)
  if (siteIds.length !== 1) return null
  return {
    siteId: siteIds[0],
    createdFrom: params.get('created_from') || undefined,
    createdTo: params.get('created_to') || undefined,
  }
}

export function costShareConfigToDraft(config: OAuthCostShareConfig): OAuthCostShareConfigDraft {
  return {
    plus: planConfigToDraft(config.plus),
    pro_lite: planConfigToDraft(config.pro_lite),
    pro: planConfigToDraft(config.pro),
  }
}

export function draftToCostShareConfig(draft: OAuthCostShareConfigDraft): OAuthCostShareConfig {
  return {
    plus: draftToPlanConfig(draft.plus),
    pro_lite: draftToPlanConfig(draft.pro_lite),
    pro: draftToPlanConfig(draft.pro),
  }
}

export function buildCostSharePieSlices(data: OAuthCostShareData, unallocatedLabel = '未分摊费用'): OAuthCostSharePieSlice[] {
  if (!data.supported) return []
  const slices = data.items
    .filter((item) => item.allocated_cost > 0)
    .map((item) => ({
      name: item.name,
      value: item.allocated_cost,
      usage_cost: item.usage_cost,
      usage_share: item.usage_share,
      allocated_cost: item.allocated_cost,
      is_unallocated: false,
    }))
  if (!data.over_quota && data.unallocated_cost > 0) {
    slices.push({
      name: unallocatedLabel,
      value: data.unallocated_cost,
      usage_cost: 0,
      usage_share: Math.max(1 - data.total_usage_ratio, 0),
      allocated_cost: data.unallocated_cost,
      is_unallocated: true,
    })
  }
  return slices
}

export function costShareWarning(response?: OAuthCostShareResponse | null) {
  const warning = response?.meta?.speed_deng_warning?.trim()
  return warning || undefined
}

function planConfigToDraft(config: OAuthCostShareConfig['plus']) {
  return {
    single_quota: String(config.single_quota),
    reset_count: String(config.reset_count),
    account_fee: String(config.account_fee),
  }
}

function draftToPlanConfig(draft: OAuthCostShareConfigDraft['plus']) {
  return {
    single_quota: nonNegativeNumber(draft.single_quota),
    reset_count: Math.trunc(nonNegativeNumber(draft.reset_count)),
    account_fee: nonNegativeNumber(draft.account_fee),
  }
}

function nonNegativeNumber(value: string) {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : 0
}
