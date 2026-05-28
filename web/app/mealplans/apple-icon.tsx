import { ImageResponse } from 'next/og'

export const size = { width: 180, height: 180 }
export const contentType = 'image/png'

export default function AppleIcon() {
  return new ImageResponse(
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="180" height="180">
      <rect width="32" height="32" rx="6" fill="#d97706" />
      <circle cx="16" cy="18" r="8" stroke="white" stroke-width="2" fill="none" />
      <circle cx="16" cy="18" r="4" stroke="white" stroke-width="1.5" fill="none" />
      <path d="M8 18h16" stroke="white" stroke-width="1.5" />
      <path
        d="M10 11c1.5-3 7-4 11.5-1.5"
        stroke="white"
        stroke-width="2"
        fill="none"
        stroke-linecap="round"
      />
    </svg>,
    { ...size }
  )
}
