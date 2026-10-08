import { ProfileService } from '../bindings/github.com/skemono/Xorcom-Tools'
import type { ProfileView } from '../bindings/github.com/skemono/Xorcom-Tools'

// Shared state: the profiles view and whether a tool is running.
export const app = $state({
  active: '',
  profiles: [] as ProfileView[],
  recovered: '',
  busy: false,
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

// Wails rejects calls with an Error carrying the Go error text.
export function errText(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}
