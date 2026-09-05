import type { SpeedDengStatus } from './types'

export function speedDengButtonMode(status?: SpeedDengStatus | null): 'start' | 'stop' {
  return status?.active ? 'stop' : 'start'
}

export function speedDengStopReasonKey(reason?: string | null) {
  switch (reason) {
    case 'manual': return 'manual'
    case 'weekly_quota_recovered': return 'weeklyQuotaRecovered'
    case 'startup_quota_recovered': return 'startupQuotaRecovered'
    default: return 'unknown'
  }
}
