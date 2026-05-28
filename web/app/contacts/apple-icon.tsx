import { ImageResponse } from 'next/og'

export const size = { width: 180, height: 180 }
export const contentType = 'image/png'

export default function AppleIcon() {
  return new ImageResponse(
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="180" height="180">
      <rect width="32" height="32" rx="6" fill="#4f46e5" />
      <circle cx="16" cy="12" r="5" stroke="white" stroke-width="2" fill="none" />
      <path
        d="M6 27c0-5.5 4.5-10 10-10s10 4.5 10 10"
        stroke="white"
        stroke-width="2"
        fill="none"
        stroke-linecap="round"
      />
    </svg>,
    { ...size }
  )
}
