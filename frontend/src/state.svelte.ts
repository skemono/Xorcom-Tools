import { ProfileService } from '../bindings/github.com/skemono/Xorcom-Tools'
import type { ProfileView } from '../bindings/github.com/skemono/Xorcom-Tools'

export type Toast = {
  id: number
  kind: 'ok' | 'fail' | 'info' | 'update'
  text: string
  action?: { label: string; run: () => void }
}

// Shared state: the profiles view, what the app is doing, and the notices on screen.
export const app = $state({
  active: '',
  profiles: [] as ProfileView[],
  recovered: '',
  busy: false, // a tool is WRITING to the PBX: navigation and the PBX switch lock until it ends
  calls: 0, // PBX calls in flight (reads and writes): the PBX sign says it is talking to the PBX
  toasts: [] as Toast[],
})

export async function reload() {
  const v = await ProfileService.List()
  app.active = v.active
  app.profiles = v.profiles ?? []
  app.recovered = v.recovered
}

export async function setActive(id: string) {
  await ProfileService.SetActive(id)
  await reload()
}

// Switch the PBX every tool acts on, and say so: which PBX is active must never be a surprise.
export async function switchPBX(id: string) {
  try {
    await setActive(id)
    notify('ok', `PBX activa: ${app.profiles.find((v) => v.profile.id === id)?.profile.name ?? id}`)
  } catch (e) {
    notify('fail', `No se pudo cambiar de PBX: ${errText(e)}`)
  }
}

// Wraps a call that talks to the PBX, so the PBX sign shows the conversation while it lasts.
export async function talk<T>(call: Promise<T>): Promise<T> {
  app.calls++
  try {
    return await call
  } finally {
    app.calls--
  }
}

let nextToast = 0
// A notice in the corner. ms = 0 keeps it until dismissed (failures and updates).
export function notify(kind: Toast['kind'], text: string, ms = kind === 'ok' || kind === 'info' ? 5000 : 0, action?: Toast['action']) {
  const id = ++nextToast
  app.toasts.push({ id, kind, text, action })
  if (ms) setTimeout(() => dismiss(id), ms)
  return id
}

export function dismiss(id: number) {
  app.toasts = app.toasts.filter((t) => t.id !== id)
}

// Wails rejects calls with an Error carrying the Go error text.
export function errText(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}
