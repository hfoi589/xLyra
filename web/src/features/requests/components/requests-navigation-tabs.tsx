import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

export function RequestsNavigationTabs({ active }: { active: 'records' | 'charts' }) {
  const navigate = useNavigate()
  const { t } = useTranslation('request-charts')

  return (
    <Tabs
      value={active}
      onValueChange={(value) => {
        navigate(value === 'charts' ? '/requests/charts' : '/requests')
      }}
    >
      <TabsList className="w-fit min-w-64 max-w-full">
        <TabsTrigger value="records">{t('tabs.records')}</TabsTrigger>
        <TabsTrigger value="charts">{t('tabs.charts')}</TabsTrigger>
      </TabsList>
    </Tabs>
  )
}
