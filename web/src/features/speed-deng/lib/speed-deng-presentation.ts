import type { SpeedDengStatus } from './types'

export type SpeedDengToolbarMode = 'start' | 'view-stop'

export type SpeedDengFirstQuotaCheckState = {
  firstQuotaCheckAt: string | null
  waiting: boolean
  remainingMs: number
}

export function speedDengToolbarMode(status?: SpeedDengStatus | null): SpeedDengToolbarMode {
  return status?.active ? 'view-stop' : 'start'
}

export function speedDengFirstQuotaCheckState(
  status?: SpeedDengStatus | null,
  now = Date.now(),
): SpeedDengFirstQuotaCheckState {
  const firstQuotaCheckAt = status?.first_quota_check_at ?? null
  if (!firstQuotaCheckAt) {
    return { firstQuotaCheckAt: null, waiting: false, remainingMs: 0 }
  }

  const target = new Date(firstQuotaCheckAt).getTime()
  if (Number.isNaN(target)) {
    return { firstQuotaCheckAt, waiting: false, remainingMs: 0 }
  }

  const remainingMs = Math.max(0, target - now)
  return {
    firstQuotaCheckAt,
    waiting: remainingMs > 0,
    remainingMs,
  }
}

export function formatSpeedDengRemainingWait(
  remainingMs: number,
  language: string,
) {
  if (remainingMs <= 0) return null
  const totalSeconds = Math.ceil(remainingMs / 1000)
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  const locale = language.startsWith('zh')
    ? 'zh-CN'
    : language.startsWith('ja') || language.startsWith('jp')
      ? 'ja-JP'
      : 'en-US'
  const parts = new Intl.NumberFormat(locale, { maximumFractionDigits: 0 })
  const formattedMinutes = minutes > 0 ? `${parts.format(minutes)}m` : ''
  const formattedSeconds = `${parts.format(seconds)}s`
  return formattedMinutes ? `${formattedMinutes} ${formattedSeconds}` : formattedSeconds
}
