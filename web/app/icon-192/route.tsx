import { ImageResponse } from 'next/og'

export function GET() {
  return new ImageResponse(
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="192" height="192">
      <rect width="32" height="32" rx="6" fill="#7c3aed" />
      <path
        fill="white"
        d="M21 7a5 5 0 0 0-4.8 6.3l-7.8 7.8a1.5 1.5 0 1 0 2.1 2.1l7.8-7.8A5 5 0 0 0 26 11l-2.8 2.8-2-.5-.5-2 2.8-2.8A5 5 0 0 0 21 7z"
      />
    </svg>,
    { width: 192, height: 192 }
  )
}
