import type { Component } from 'svelte'
import Conexiones from './modules/Conexiones.svelte'
import Pines from './modules/Pines.svelte'

// Tool registry (frontend side): one line per tool, in directory order, each with its pictogram.
export const modules: { id: string; label: string; icon: string; component: Component }[] = [
  { id: 'conexiones', label: 'Conexiones', icon: 'plug', component: Conexiones },
  { id: 'pines', label: 'PINes masivos', icon: 'keypad', component: Pines },
]
