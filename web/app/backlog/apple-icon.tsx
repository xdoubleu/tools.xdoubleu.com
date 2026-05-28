import { ImageResponse } from 'next/og'

export const size = { width: 180, height: 180 }
export const contentType = 'image/png'

export default function AppleIcon() {
  return new ImageResponse(
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="180" height="180">
      <rect width="32" height="32" rx="6" fill="#0d9488" />
      <line
        x1="8"
        y1="10"
        x2="24"
        y2="10"
        stroke="white"
        stroke-width="2.5"
        stroke-linecap="round"
      />
      <line
        x1="8"
        y1="16"
        x2="21"
        y2="16"
        stroke="white"
        stroke-width="2.5"
        stroke-linecap="round"
      />
      <line
        x1="8"
        y1="22"
        x2="17"
        y2="22"
        stroke="white"
        stroke-width="2.5"
        stroke-linecap="round"
      />
    </svg>,
    { ...size }
  )
}
