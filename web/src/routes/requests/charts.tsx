import { RequestsNavigationTabs } from '@/features/requests/components/requests-navigation-tabs'
import { RequestsAnalyticsWorkspace } from '@/features/requests/components/requests-analytics-workspace'

export function RequestsChartsPage() {
  return (
    <div className="space-y-5">
      <RequestsNavigationTabs active="charts" />
      <RequestsAnalyticsWorkspace />
    </div>
  )
}
