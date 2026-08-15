import { ImageResponse } from 'next/og'

export const size = { width: 180, height: 180 }
export const contentType = 'image/png'

export default function AppleIcon() {
  return new ImageResponse(
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="180" height="180">
      <rect width="32" height="32" rx="6" fill="#7c3aed" />
      <rect x="7" y="7" width="8" height="8" rx="1.5" fill="white" />
      <rect x="17" y="7" width="8" height="8" rx="1.5" fill="white" fill-opacity="0.6" />
      <rect x="7" y="17" width="8" height="8" rx="1.5" fill="white" fill-opacity="0.6" />
      <rect x="17" y="17" width="8" height="8" rx="1.5" fill="white" />
    </svg>,
    { ...size }
  )
}
