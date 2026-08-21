import { useEffect } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { RequestsNavigationTabs } from '@/features/requests/components/requests-navigation-tabs'
import { RequestsWorkspace } from '@/features/requests/components/requests-workspace'

export function RequestsPage() {
  const location = useLocation()
  const navigate = useNavigate()

  useEffect(() => {
    if (!location.search) return
    navigate({ pathname: location.pathname, hash: location.hash }, { replace: true })
  }, [location.hash, location.pathname, location.search, navigate])

  return (
    <div className="space-y-5">
      <RequestsNavigationTabs active="records" />
      <RequestsWorkspace initialSearch={location.search} />
    </div>
  )
}
