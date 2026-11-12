#!/usr/bin/env node
/**
 * Runs mobile-audit.mjs against a local api (own database, serving web/dist)
 * signed in as a seeded admin. Needs Postgres, Go and psql; pages show empty states.
 * Usage: npm run mobile:audit:local -- /trains [audit flags]
 */
/* global fetch, setTimeout */
import { spawn, spawnSync } from 'node:child_process'
import { existsSync, mkdtempSync, statSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const webDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const apiDir = path.resolve(webDir, '../api')
const PORT = process.env.MOBILE_AUDIT_PORT ?? '8123'
const BASE_URL = `http://localhost:${PORT}`
const PG_ADMIN_DSN = process.env.MOBILE_AUDIT_PG_DSN ?? 'postgres://postgres@localhost/postgres'
const DB_NAME = 'mobile_audit'
const DB_DSN = PG_ADMIN_DSN.replace(/\/[^/]*$/, `/${DB_NAME}`)
const EMAIL = 'audit@example.com'
const PASSWORD = 'audit-pass-123'
// bcrypt (cost 10) of PASSWORD, so seeding needs no hashing tool.
const PASSWORD_HASH = '$2y$10$m7SQZkzf04sMyrsT.hhUI.S5pOzQoV.f0edK04dgYnHWUYHazu9P.'
const BOOT_TIMEOUT_MS = 180_000

function psql(dsn, sql) {
  const r = spawnSync('psql', [dsn, '-v', 'ON_ERROR_STOP=1', '-tAc', sql], { encoding: 'utf8' })
  if (r.error?.code === 'ENOENT') throw new Error('psql not found; install the PostgreSQL client')
  if (r.status !== 0) throw new Error(`psql failed: ${r.stderr.trim() || r.stdout.trim()}`)
  return r.stdout.trim()
}

function ensureDatabase() {
  if (psql(PG_ADMIN_DSN, `select 1 from pg_database where datname='${DB_NAME}'`) !== '1') {
    psql(PG_ADMIN_DSN, `create database ${DB_NAME}`)
  }
}

function newestSourceMtime() {
  const r = spawnSync('git', ['ls-files', '-z', '--', '.'], { cwd: webDir, encoding: 'utf8' })
  return r.stdout
    .split('\0')
    .filter((f) => f && !f.startsWith('__tests__/') && existsSync(path.join(webDir, f)))
    .reduce((max, f) => Math.max(max, statSync(path.join(webDir, f)).mtimeMs), 0)
}

function ensureBuild() {
  const index = path.join(webDir, 'dist/index.html')
  if (existsSync(index) && statSync(index).mtimeMs >= newestSourceMtime()) return
  console.error('building web/dist …')
  const r = spawnSync('npm', ['run', 'build'], { cwd: webDir, encoding: 'utf8' })
  if (r.status !== 0) throw new Error(`npm run build failed:\n${r.stdout.slice(-2000)}${r.stderr}`)
}

async function waitFor(url, child) {
  const deadline = Date.now() + BOOT_TIMEOUT_MS
  while (Date.now() < deadline) {
    if (child.exitCode !== null) throw new Error(`api exited early (code ${child.exitCode})`)
    try {
      if ((await fetch(url)).ok) return
    } catch {
      // not listening yet
    }
    await new Promise((r) => setTimeout(r, 500))
  }
  throw new Error(`api did not answer ${url} within ${BOOT_TIMEOUT_MS / 1000}s`)
}

function seedAdmin() {
  psql(
    DB_DSN,
    `with u as (
       insert into auth.users (email, password_hash) values ('${EMAIL}', '${PASSWORD_HASH}')
       on conflict (email) do update set password_hash = excluded.password_hash
       returning id, email)
     insert into global.app_users (id, email, role)
     select id::text, email, 'admin' from u
     on conflict (id) do update set role = 'admin'`
  )
}

async function signedInStorageState() {
  const res = await fetch(`${BASE_URL}/api/auth.v1.AuthService/SignIn`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Connect-Protocol-Version': '1' },
    body: JSON.stringify({ email: EMAIL, password: PASSWORD })
  })
  if (!res.ok) throw new Error(`sign-in failed: HTTP ${res.status} ${await res.text()}`)
  const cookies = res.headers
    .getSetCookie()
    .map((c) => c.split(';')[0].split('='))
    .filter(([, value]) => value)
    .map(([name, ...rest]) => ({
      name,
      value: rest.join('='),
      domain: 'localhost',
      path: '/',
      expires: -1,
      httpOnly: true,
      secure: false,
      sameSite: 'Lax'
    }))
  if (cookies.length === 0) throw new Error('sign-in set no session cookie')
  const file = path.join(mkdtempSync(path.join(tmpdir(), 'mobile-audit-')), 'state.json')
  writeFileSync(file, JSON.stringify({ cookies, origins: [] }))
  return file
}

function startApi() {
  const env = {
    ...process.env,
    ENV: 'development',
    PORT,
    THROTTLE: 'false',
    API_URL: BASE_URL,
    WEB_URL: BASE_URL,
    DB_DSN,
    JWT_SECRET: 'mobile-audit-jwt-secret-0123456789abcdef',
    OAUTH_HMAC_SECRET: 'mobile-audit-hmac-secret-0123456789abcdef',
    WEB_DIR: path.join(webDir, 'dist')
  }
  // A built binary, not `go run`, so stopping it stops the server itself.
  const bin = path.join(tmpdir(), 'mobile-audit-api')
  const build = spawnSync('go', ['build', '-o', bin, './cmd/api'], {
    cwd: apiDir,
    stdio: 'inherit'
  })
  if (build.status !== 0) throw new Error('go build failed')
  return spawn(bin, [], { cwd: apiDir, env, stdio: ['ignore', 'ignore', 'pipe'] })
}

async function main() {
  ensureDatabase()
  ensureBuild()
  const api = startApi()
  let apiErr = ''
  api.stderr.on('data', (d) => (apiErr = (apiErr + d).slice(-4000)))
  const stop = () => api.kill('SIGTERM')
  process.on('exit', stop)
  try {
    await waitFor(`${BASE_URL}/`, api)
    seedAdmin()
    const storageState = await signedInStorageState()
    const audit = spawnSync(
      process.execPath,
      [
        path.join(webDir, 'scripts/mobile-audit.mjs'),
        ...process.argv.slice(2),
        '--base-url',
        BASE_URL,
        '--storage-state',
        storageState
      ],
      { stdio: 'inherit' }
    )
    return audit.status ?? 1
  } catch (err) {
    console.error(`${err.message}\n${apiErr}`)
    return 1
  } finally {
    stop()
  }
}

process.exit(await main())
