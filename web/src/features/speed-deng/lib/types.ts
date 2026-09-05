export type SpeedDengQuotaCheck = {
  eligible_count: number
  checked_count: number
  skipped_count: number
  recovered: boolean
  warning?: string
}

export type SpeedDengStatus = {
  active: boolean
  state: string
  session_id?: string | null
  started_at?: string | null
  stopped_at?: string | null
  stop_reason?: string | null
  first_quota_check_at?: string | null
  event_count: number
  last_quota_check_at?: string | null
  quota_check?: SpeedDengQuotaCheck
}
