import { ImageResponse } from 'next/og'

export const size = { width: 180, height: 180 }
export const contentType = 'image/png'

export default function AppleIcon() {
  return new ImageResponse(
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="180" height="180">
      <rect width="32" height="32" rx="6" fill="#16a34a" />
      <rect x="7" y="9" width="18" height="16" rx="2" stroke="white" stroke-width="2" fill="none" />
      <line x1="7" y1="14" x2="25" y2="14" stroke="white" stroke-width="2" />
      <line x1="12" y1="7" x2="12" y2="11" stroke="white" stroke-width="2" stroke-linecap="round" />
      <line x1="20" y1="7" x2="20" y2="11" stroke="white" stroke-width="2" stroke-linecap="round" />
      <circle cx="12" cy="19" r="1.5" fill="white" />
      <circle cx="16" cy="19" r="1.5" fill="white" />
      <circle cx="20" cy="19" r="1.5" fill="white" />
    </svg>,
    { ...size }
  )
}
