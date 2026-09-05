import { apiFetch } from '@/lib/http'
import type { SpeedDengStatus } from '../lib/types'

export const speedDengQueryKeys = {
  all: ['speed-deng'] as const,
  status: () => [...speedDengQueryKeys.all, 'status'] as const,
}

export async function getSpeedDengStatus() {
  return apiFetch<SpeedDengStatus>('/api/v1/settings/speed-deng')
}

export async function startSpeedDeng() {
  return apiFetch<SpeedDengStatus>('/api/v1/settings/speed-deng/start', { method: 'POST' })
}

export async function stopSpeedDeng() {
  return apiFetch<SpeedDengStatus>('/api/v1/settings/speed-deng/stop', { method: 'POST' })
}
