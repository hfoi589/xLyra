import { describe, expect, it } from 'vitest'
import { speedDengStopReasonKey } from './speed-deng-utils'

describe('speed-deng status presentation', () => {
  it('maps automatic and manual stop reasons to stable translation keys', () => {
    expect(speedDengStopReasonKey('manual')).toBe('manual')
    expect(speedDengStopReasonKey('weekly_quota_recovered')).toBe('weeklyQuotaRecovered')
    expect(speedDengStopReasonKey('startup_quota_recovered')).toBe('startupQuotaRecovered')
    expect(speedDengStopReasonKey('other')).toBe('unknown')
  })
})
