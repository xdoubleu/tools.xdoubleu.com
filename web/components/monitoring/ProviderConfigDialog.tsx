'use client'

import { useEffect, useState } from 'react'
import { useProviderOptions, useSetProviderConfig } from '@/hooks/useMonitoring'
import type { ProviderConfigInput } from '@/hooks/useMonitoring'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { Checkbox } from '@/components/ui/checkbox'

const PROVIDER_LABELS: Record<string, string> = {
  github: 'GitHub',
  sentry: 'Sentry',
  digitalocean: 'DigitalOcean'
}

interface ProviderConfigDialogProps {
  provider: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

export default function ProviderConfigDialog({
  provider,
  open,
  onOpenChange
}: ProviderConfigDialogProps) {
  const fetchOptions = useProviderOptions()
  const setProviderConfig = useSetProviderConfig()

  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const [repos, setRepos] = useState<string[]>([])
  const [repo, setRepo] = useState('')

  const [apps, setApps] = useState<string[]>([])
  const [app, setApp] = useState('')

  const [orgs, setOrgs] = useState<string[]>([])
  const [org, setOrg] = useState('')
  const [projects, setProjects] = useState<string[]>([])
  const [selectedProjects, setSelectedProjects] = useState<string[]>([])

  useEffect(() => {
    if (!open) return
    setError('')
    setRepo('')
    setApp('')
    setOrg('')
    setProjects([])
    setSelectedProjects([])
    setLoading(true)

    fetchOptions(provider)
      .then((resp) => {
        setRepos(resp.repos)
        setApps(resp.apps)
        setOrgs(resp.sentryOrgs)
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load options.'))
      .finally(() => setLoading(false))
  }, [open, provider, fetchOptions])

  const handleOrgChange = (nextOrg: string) => {
    setOrg(nextOrg)
    setProjects([])
    setSelectedProjects([])
    if (!nextOrg) return

    setLoading(true)
    fetchOptions(provider, nextOrg)
      .then((resp) => setProjects(resp.sentryProjects))
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load projects.'))
      .finally(() => setLoading(false))
  }

  const toggleProject = (project: string) => {
    setSelectedProjects((prev) =>
      prev.includes(project) ? prev.filter((p) => p !== project) : [...prev, project]
    )
  }

  const canSave =
    provider === 'github'
      ? repo !== ''
      : provider === 'digitalocean'
        ? app !== ''
        : org !== '' && selectedProjects.length > 0

  const handleSave = async () => {
    let config: ProviderConfigInput
    if (provider === 'github') {
      config = { config: { case: 'github', value: { repo } } }
    } else if (provider === 'digitalocean') {
      config = { config: { case: 'digitalocean', value: { appId: app } } }
    } else {
      config = {
        config: { case: 'sentry', value: { org, projects: selectedProjects } }
      }
    }

    setSaving(true)
    setError('')
    try {
      await setProviderConfig(provider, config)
      onOpenChange(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to save configuration.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Configure {PROVIDER_LABELS[provider] ?? provider}</DialogTitle>
          <DialogClose aria-label="Close">x</DialogClose>
        </DialogHeader>

        {loading ? (
          <p className="py-8 text-center text-sm text-muted">Loading…</p>
        ) : (
          <div className="space-y-4">
            {provider === 'github' && (
              <div>
                <label className="mb-1 block text-sm font-medium text-fg">Repository</label>
                <Select value={repo} onChange={(e) => setRepo(e.target.value)}>
                  <option value="">Select a repository…</option>
                  {repos.map((r) => (
                    <option key={r} value={r}>
                      {r}
                    </option>
                  ))}
                </Select>
              </div>
            )}

            {provider === 'digitalocean' && (
              <div>
                <label className="mb-1 block text-sm font-medium text-fg">App</label>
                <Select value={app} onChange={(e) => setApp(e.target.value)}>
                  <option value="">Select an app…</option>
                  {apps.map((a) => (
                    <option key={a} value={a}>
                      {a}
                    </option>
                  ))}
                </Select>
              </div>
            )}

            {provider === 'sentry' && (
              <>
                <div>
                  <label className="mb-1 block text-sm font-medium text-fg">Organization</label>
                  <Select value={org} onChange={(e) => handleOrgChange(e.target.value)}>
                    <option value="">Select an organization…</option>
                    {orgs.map((o) => (
                      <option key={o} value={o}>
                        {o}
                      </option>
                    ))}
                  </Select>
                </div>

                {org !== '' && (
                  <div>
                    <label className="mb-1 block text-sm font-medium text-fg">Projects</label>
                    {projects.length === 0 ? (
                      <p className="text-sm text-muted">No projects found in {org}.</p>
                    ) : (
                      <div className="flex flex-col gap-2">
                        {projects.map((p) => (
                          <Checkbox
                            key={p}
                            id={`project-${p}`}
                            label={p}
                            checked={selectedProjects.includes(p)}
                            onChange={() => toggleProject(p)}
                          />
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </>
            )}
          </div>
        )}

        {error && <p className="mt-3 text-sm text-danger">{error}</p>}

        <div className="mt-6 flex justify-end gap-2">
          <Button variant="ghost" disabled={saving} onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={() => void handleSave()} disabled={saving || loading || !canSave}>
            {saving ? 'Saving…' : 'Save'}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
