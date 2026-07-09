import { NextResponse } from 'next/server'

export const dynamic = 'force-dynamic'

export function GET() {
  return NextResponse.json(
    { release: process.env.RELEASE ?? 'dev' },
    { headers: { 'Cache-Control': 'no-store' } }
  )
}
