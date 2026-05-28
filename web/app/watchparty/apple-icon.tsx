import { ImageResponse } from 'next/og'

export const size = { width: 180, height: 180 }
export const contentType = 'image/png'

export default function AppleIcon() {
  return new ImageResponse(
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="180" height="180">
      <rect width="32" height="32" rx="6" fill="#dc2626" />
      <circle cx="16" cy="16" r="9" stroke="white" stroke-width="2" fill="none" />
      <polygon points="13,11 13,21 22,16" fill="white" />
    </svg>,
    { ...size }
  )
}
