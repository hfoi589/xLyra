import { RequestsNavigationTabs } from '@/features/requests/components/requests-navigation-tabs'
import { RequestsAnalyticsWorkspace } from '@/features/requests/components/requests-analytics-workspace'
import { OAuthCostSharePanel } from '@/features/oauth-cost-share/components/oauth-cost-share-panel'

export function RequestsChartsPage() {
  return (
    <div className="space-y-5">
      <RequestsNavigationTabs active="charts" />
      <RequestsAnalyticsWorkspace />
      <OAuthCostSharePanel />
    </div>
  )
}
