import type { Component } from 'svelte'
import Conexiones from './modules/Conexiones.svelte'

// Tool registry (frontend side): one line per form, in pad order.
export const modules: { code: string; id: string; label: string; component: Component }[] = [
  { code: 'F-01', id: 'conexiones', label: 'Conexiones', component: Conexiones },
]
