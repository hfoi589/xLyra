import { describe, expect, it } from 'vitest'
import {
  formatSpeedDengRemainingWait,
  speedDengFirstQuotaCheckState,
  speedDengToolbarMode,
} from './speed-deng-presentation'
import type { SpeedDengStatus } from './types'

const inactive: SpeedDengStatus = {
  active: false,
  state: 'inactive',
  event_count: 0,
  quota_check: { eligible_count: 0, checked_count: 0, skipped_count: 0, recovered: false },
}

describe('speed-deng presentation', () => {
  it('shows the run/view-stop split by session state', () => {
    expect(speedDengToolbarMode(inactive)).toBe('start')
    expect(speedDengToolbarMode({ ...inactive, active: true, state: 'active' })).toBe('view-stop')
  })

  it('tracks the first quota check delay window', () => {
    const state = speedDengFirstQuotaCheckState(
      { ...inactive, first_quota_check_at: '2026-09-05T10:10:00.000Z' },
      Date.parse('2026-09-05T10:00:00.000Z'),
    )

    expect(state).toEqual({
      firstQuotaCheckAt: '2026-09-05T10:10:00.000Z',
      waiting: true,
      remainingMs: 600_000,
    })
    expect(formatSpeedDengRemainingWait(state.remainingMs, 'zh-CN')).toBe('10m 0s')
  })

  it('treats malformed or elapsed quota timestamps as ready', () => {
    expect(speedDengFirstQuotaCheckState({ ...inactive, first_quota_check_at: 'bad-time' })).toEqual({
      firstQuotaCheckAt: 'bad-time',
      waiting: false,
      remainingMs: 0,
    })
    expect(formatSpeedDengRemainingWait(0, 'en-US')).toBeNull()
  })
})
