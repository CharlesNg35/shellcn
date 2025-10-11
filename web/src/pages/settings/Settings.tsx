import { useState } from 'react'
import { PageHeader } from '@/components/layout/PageHeader'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/Tabs'
import { AccountSettingsPanel } from '@/components/settings/AccountSettingsPanel'
import { SecuritySettingsPanel } from '@/components/settings/SecuritySettingsPanel'
import { AppearanceSettingsPanel } from '@/components/settings/AppearanceSettingsPanel'

const SETTINGS_TABS = [
  {
    value: 'account',
    label: 'Account',
    description: 'Profile details, identity, and localization preferences.',
  },
  {
    value: 'security',
    label: 'Security',
    description: 'Password management, MFA enrollment, and recovery options.',
  },
  {
    value: 'appearance',
    label: 'Appearance',
    description: 'Theme preferences for the web interface.',
  },
] as const

export function Settings() {
  const [activeTab, setActiveTab] = useState<(typeof SETTINGS_TABS)[number]['value']>('account')

  const activeDescription = SETTINGS_TABS.find((tab) => tab.value === activeTab)?.description

  return (
    <div className="space-y-6">
      <PageHeader
        title="Settings & Preferences"
        description="Customize your ShellCN experience, manage profile details, and enforce account security."
      />

      <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as typeof activeTab)}>
        <TabsList className="flex-wrap gap-2 bg-muted/40 p-1">
          {SETTINGS_TABS.map((tab) => (
            <TabsTrigger key={tab.value} value={tab.value} className="px-4">
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>
        {activeDescription ? (
          <p className="text-sm text-muted-foreground my-3">{activeDescription}</p>
        ) : null}

        <TabsContent value="account" className="space-y-6">
          <AccountSettingsPanel />
        </TabsContent>

        <TabsContent value="security" className="space-y-6">
          <SecuritySettingsPanel />
        </TabsContent>

        <TabsContent value="appearance" className="space-y-6">
          <AppearanceSettingsPanel />
        </TabsContent>
      </Tabs>
    </div>
  )
}
