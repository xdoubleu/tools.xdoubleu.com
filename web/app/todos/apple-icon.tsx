import { ImageResponse } from 'next/og'

export const size = { width: 180, height: 180 }
export const contentType = 'image/png'

export default function AppleIcon() {
  return new ImageResponse(
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="180" height="180">
      <rect width="32" height="32" rx="6" fill="#2563eb" />
      <rect x="8" y="8" width="16" height="16" rx="2" stroke="white" stroke-width="2" fill="none" />
      <polyline
        points="11,16 14,19 21,12"
        stroke="white"
        stroke-width="2.5"
        fill="none"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </svg>,
    { ...size }
  )
}
