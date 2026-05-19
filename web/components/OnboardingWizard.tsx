'use client'

import { useState } from 'react'
import { useSettings } from '@/hooks/useSettings'

interface OnboardingWizardProps {
  onFinish: () => void
}

type Step = 1 | 2

export default function OnboardingWizard({ onFinish }: OnboardingWizardProps) {
  useSettings()
  const [step, setStep] = useState<Step>(1)
  const [steamApiKey, setSteamApiKey] = useState('')
  const [steamUserId, setSteamUserId] = useState('')
  const [hardcoverApiKey, setHardcoverApiKey] = useState('')

  const handleNext = () => {
    if (step === 1) {
      setStep(2)
    }
  }

  const handleSkip = () => {
    onFinish()
  }

  const handleFinish = async () => {
    // In a real implementation, this would save the settings
    // For now, just call onFinish
    onFinish()
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
      <div className="bg-card rounded-lg shadow-lg max-w-md w-full p-6 space-y-4">
        <div className="flex gap-2 mb-6">
          <div className={`h-2 flex-1 rounded ${step >= 1 ? 'bg-blue-600' : 'bg-border'}`} />
          <div className={`h-2 flex-1 rounded ${step >= 2 ? 'bg-blue-600' : 'bg-border'}`} />
        </div>

        {step === 1 && (
          <div className="space-y-4">
            <h2 className="text-lg font-semibold">Setup Steam Integration</h2>
            <div>
              <label className="block text-sm font-medium text-subtle mb-1">Steam API Key</label>
              <input
                type="password"
                value={steamApiKey}
                onChange={(e) => setSteamApiKey(e.target.value)}
                placeholder="Enter your Steam API key"
                className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <p className="text-xs text-muted mt-1">
                Get it from{' '}
                <a
                  href="https://steamcommunity.com/dev/apikey"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-blue-600 hover:underline"
                >
                  steamcommunity.com/dev/apikey
                </a>
              </p>
            </div>

            <div>
              <label className="block text-sm font-medium text-subtle mb-1">Steam User ID</label>
              <input
                type="text"
                value={steamUserId}
                onChange={(e) => setSteamUserId(e.target.value)}
                placeholder="Your Steam User ID"
                className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <p className="text-xs text-muted mt-1">
                Find it at{' '}
                <a
                  href="https://steamid.io"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-blue-600 hover:underline"
                >
                  steamid.io
                </a>
              </p>
            </div>
          </div>
        )}

        {step === 2 && (
          <div className="space-y-4">
            <h2 className="text-lg font-semibold">Setup Recipe Integration</h2>
            <div>
              <label className="block text-sm font-medium text-subtle mb-1">
                Hardcover API Key
              </label>
              <input
                type="password"
                value={hardcoverApiKey}
                onChange={(e) => setHardcoverApiKey(e.target.value)}
                placeholder="Enter your Hardcover API key"
                className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <p className="text-xs text-muted mt-1">
                Get it from your{' '}
                <a
                  href="https://hardcover.app/settings/api"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-blue-600 hover:underline"
                >
                  Hardcover settings
                </a>
              </p>
            </div>
          </div>
        )}

        <div className="flex gap-2 pt-4">
          <button
            onClick={handleSkip}
            className="flex-1 px-4 py-2 bg-surface text-fg rounded hover:bg-border"
          >
            Skip
          </button>
          {step === 1 && (
            <button
              onClick={handleNext}
              className="flex-1 px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
            >
              Next
            </button>
          )}
          {step === 2 && (
            <>
              <button
                onClick={() => setStep(1)}
                className="flex-1 px-4 py-2 bg-surface text-fg rounded hover:bg-border"
              >
                Back
              </button>
              <button
                onClick={handleFinish}
                className="flex-1 px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
              >
                Finish
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
