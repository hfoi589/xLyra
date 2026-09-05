import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Activity, LoaderCircle, Power, Square } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { toast } from '@/lib/toast'
import { oauthCostShareQueryKeys } from '@/features/oauth-cost-share/api/oauth-cost-share'
import {
  getSpeedDengStatus,
  speedDengQueryKeys,
  startSpeedDeng,
  stopSpeedDeng,
} from '../api/speed-deng'
import {
  formatSpeedDengRemainingWait,
  speedDengFirstQuotaCheckState,
  speedDengToolbarMode,
} from '../lib/speed-deng-presentation'
import { speedDengStopReasonKey } from '../lib/speed-deng-utils'
import type { SpeedDengStatus } from '../lib/types'

export function SpeedDengControl() {
  const { t, i18n } = useTranslation('speed-deng')
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const statusQuery = useQuery({
    queryKey: speedDengQueryKeys.status(),
    queryFn: getSpeedDengStatus,
    refetchInterval: 5_000,
    refetchIntervalInBackground: true,
  })

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: speedDengQueryKeys.status() }),
      queryClient.invalidateQueries({ queryKey: oauthCostShareQueryKeys.all }),
    ])
  }

  const startMutation = useMutation({
    mutationFn: startSpeedDeng,
    onSuccess: async (nextStatus) => {
      queryClient.setQueryData(speedDengQueryKeys.status(), nextStatus)
      setOpen(true)
      await invalidate()
      toast.success(t('toast.started'))
    },
    onError: (error) => toast.error(t('toast.startFailed'), { description: error.message }),
  })
  const stopMutation = useMutation({
    mutationFn: stopSpeedDeng,
    onSuccess: async (nextStatus) => {
      queryClient.setQueryData(speedDengQueryKeys.status(), nextStatus)
      await invalidate()
      toast.success(t('toast.stopped'))
    },
    onError: (error) => toast.error(t('toast.stopFailed'), { description: error.message }),
  })

  const status = statusQuery.data
  const mode = speedDengToolbarMode(status)
  const pending = statusQuery.isLoading || startMutation.isPending || stopMutation.isPending

  function handleButton() {
    if (mode === 'view-stop') {
      setOpen(true)
      return
    }
    startMutation.mutate()
  }

  return (
    <>
      {mode === 'view-stop' ? (
        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={handleButton}
            disabled={pending || Boolean(statusQuery.error)}
          >
            {pending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Activity className="h-4 w-4" />}
            {t('button.view')}
          </Button>
          <Button
            type="button"
            variant="destructive"
            onClick={() => stopMutation.mutate()}
            disabled={pending || Boolean(statusQuery.error)}
            aria-pressed="true"
          >
            {stopMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Square className="h-4 w-4" />}
            {t('button.stop')}
          </Button>
        </div>
      ) : (
        <Button
          type="button"
          variant="outline"
          onClick={handleButton}
          disabled={pending || Boolean(statusQuery.error)}
        >
          {startMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Power className="h-4 w-4" />}
          {t('button.start')}
        </Button>
      )}

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('dialog.title')}</DialogTitle>
            <DialogDescription>{t('dialog.description')}</DialogDescription>
          </DialogHeader>
          <DialogBody>
            <SpeedDengStatusBody status={status} language={i18n.language} />
          </DialogBody>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setOpen(false)}>
              {t('dialog.close')}
            </Button>
            {status?.active ? (
              <Button type="button" variant="destructive" onClick={() => stopMutation.mutate()} disabled={stopMutation.isPending}>
                {stopMutation.isPending ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Square className="h-4 w-4" />}
                {t('button.stop')}
              </Button>
            ) : null}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

function SpeedDengStatusBody({ status, language }: { status?: SpeedDengStatus; language: string }) {
  const { t } = useTranslation('speed-deng')
  if (!status) {
    return <div className="flex min-h-[180px] items-center justify-center text-sm text-muted-soft"><LoaderCircle className="mr-2 h-4 w-4 animate-spin" />{t('dialog.loading')}</div>
  }
  const quota = status.quota_check ?? { eligible_count: 0, checked_count: 0, skipped_count: 0, recovered: false }
  const firstQuotaCheck = speedDengFirstQuotaCheckState(status)
  const waitText = formatSpeedDengRemainingWait(firstQuotaCheck.remainingMs, language)
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 rounded-xl border border-[hsl(var(--glass-border))] bg-[hsl(var(--surface-panel))] px-4 py-3" aria-live="polite">
        <Activity className={status.active ? 'h-4 w-4 text-emerald-300' : 'h-4 w-4 text-muted-soft'} />
        <span className={status.active ? 'text-sm font-medium text-emerald-200' : 'text-sm font-medium text-muted-soft'}>
          {status.active ? t('status.active') : t('status.inactive')}
        </span>
      </div>
      <div className="grid gap-3 text-sm sm:grid-cols-2">
        <StatusValue label={t('dialog.startedAt')} value={formatTimestamp(status.started_at, language)} />
        <StatusValue label={t('dialog.eventCount')} value={status.event_count.toLocaleString()} />
        <StatusValue label={t('dialog.lastCheck')} value={formatTimestamp(status.last_quota_check_at, language)} />
        <StatusValue label={t('dialog.quotaChecked')} value={`${quota.checked_count}/${quota.eligible_count}`} />
        <StatusValue label={t('dialog.firstQuotaCheckAt')} value={formatTimestamp(firstQuotaCheck.firstQuotaCheckAt, language)} />
        <StatusValue label={t('dialog.firstQuotaWait')} value={firstQuotaCheck.waiting ? waitText ?? t('dialog.firstQuotaReady') : t('dialog.firstQuotaReady')} />
      </div>
      {quota.skipped_count > 0 ? <p className="text-xs text-amber-200">{t('dialog.quotaSkipped', { count: quota.skipped_count })}</p> : null}
      {quota.warning ? <p className="rounded-lg border border-amber-300/30 bg-amber-300/10 px-3 py-2 text-xs text-amber-100">{t('dialog.quotaWarning')}: {quota.warning}</p> : null}
      {status.stop_reason ? <p className="text-xs text-muted-soft">{t('dialog.stopReason')}: {t(`stopReasons.${speedDengStopReasonKey(status.stop_reason)}`)}</p> : null}
      {firstQuotaCheck.waiting && waitText ? <p className="text-xs text-muted-soft">{t('dialog.firstQuotaPending', { duration: waitText })}</p> : null}
    </div>
  )
}

function StatusValue({ label, value }: { label: string; value: string }) {
  return <div className="rounded-lg border border-[hsl(var(--glass-border))] px-3 py-2"><div className="text-xs text-muted-soft">{label}</div><div className="mt-1 tabular-nums text-foreground">{value}</div></div>
}

function formatTimestamp(value: string | null | undefined, language: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(language.startsWith('zh') ? 'zh-CN' : language.startsWith('ja') || language.startsWith('jp') ? 'ja-JP' : 'en-US', {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}
