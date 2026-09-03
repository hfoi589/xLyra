import { apiFetch } from '@/lib/http'
import { costShareQueryParams } from '../lib/oauth-cost-share-utils'
import type {
  OAuthCostShareConfig,
  OAuthCostShareQuery,
  OAuthCostShareResponse,
} from '../lib/types'

export const oauthCostShareQueryKeys = {
  all: ['oauth-cost-share'] as const,
  config: () => [...oauthCostShareQueryKeys.all, 'config'] as const,
  detail: (query: OAuthCostShareQuery) => [...oauthCostShareQueryKeys.all, 'detail', query] as const,
}

export async function getOAuthCostShare(query: OAuthCostShareQuery) {
  const params = costShareQueryParams(query)
  return apiFetch<OAuthCostShareResponse>(`/api/v1/requests/oauth-cost-share?${params.toString()}`)
}

export async function getOAuthCostShareConfig() {
  return apiFetch<OAuthCostShareConfig>('/api/v1/settings/oauth-cost-share')
}

export async function updateOAuthCostShareConfig(config: OAuthCostShareConfig) {
  return apiFetch<OAuthCostShareConfig>('/api/v1/settings/oauth-cost-share', {
    method: 'PUT',
    body: config,
  })
}
