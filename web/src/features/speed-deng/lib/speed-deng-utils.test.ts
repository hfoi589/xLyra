import { describe, expect, it } from 'vitest'
import { speedDengButtonMode, speedDengStopReasonKey } from './speed-deng-utils'
import type { SpeedDengStatus } from './types'

const inactive: SpeedDengStatus = { active: false, state: 'inactive', event_count: 0, quota_check: { eligible_count: 0, checked_count: 0, skipped_count: 0, recovered: false } }

describe('speed-deng status presentation', () => {
  it('uses stop mode whenever the global session is active', () => {
    expect(speedDengButtonMode({ ...inactive, active: true, state: 'active' })).toBe('stop')
    expect(speedDengButtonMode(inactive)).toBe('start')
  })

  it('maps automatic and manual stop reasons to stable translation keys', () => {
    expect(speedDengStopReasonKey('manual')).toBe('manual')
    expect(speedDengStopReasonKey('weekly_quota_recovered')).toBe('weeklyQuotaRecovered')
    expect(speedDengStopReasonKey('startup_quota_recovered')).toBe('startupQuotaRecovered')
    expect(speedDengStopReasonKey('other')).toBe('unknown')
  })
})
