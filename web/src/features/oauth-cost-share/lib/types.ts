export type OAuthCostSharePlanConfig = {
  single_quota: number
  reset_count: number
  account_fee: number
}

export type OAuthCostShareConfig = {
  plus: OAuthCostSharePlanConfig
  pro_lite: OAuthCostSharePlanConfig
  pro: OAuthCostSharePlanConfig
}

export type OAuthCostShareConfigDraft = {
  [plan in keyof OAuthCostShareConfig]: {
    single_quota: string
    reset_count: string
    account_fee: string
  }
}

export type OAuthCostShareItem = {
  name: string
  usage_cost: number
  usage_share: number
  allocated_cost: number
}

export type OAuthCostShareData = {
  supported: boolean
  unsupported_reason?: string
  site_id: string
  site_label: string
  plan_type: string
  single_quota: number
  reset_count: number
  total_quota: number
  account_fee: number
  total_usage_cost: number
  total_usage_ratio: number
  allocated_cost: number
  unallocated_cost: number
  over_quota: boolean
  items: OAuthCostShareItem[]
}

export type OAuthCostShareResponse = {
  meta: {
    range_start: string
    range_end: string
    timezone: string
    currency: string
    request_count: number
    missing_cost_requests: number
  }
  data: OAuthCostShareData
}

export type OAuthCostShareQuery = {
  siteId: string
  createdFrom?: string
  createdTo?: string
}

export type OAuthCostSharePieSlice = {
  name: string
  value: number
  usage_cost: number
  usage_share: number
  allocated_cost: number
  is_unallocated: boolean
}
