<script lang="ts">
  import { onMount } from 'svelte'
  import { UpdateService } from '../bindings/github.com/skemono/Xorcom-Tools'
  import { modules } from './modules'
  import { app, reload, setActive, errText } from './state.svelte'

  let current = $state(modules[0])
  const active = $derived(app.profiles.find((v) => v.profile.id === app.active)?.profile)
  const today = new Date().toLocaleDateString('es-GT', { day: '2-digit', month: '2-digit', year: 'numeric' })

  let version = $state('')
  type Update =
    | { s: 'idle' | 'checking' | 'current' }
    | { s: 'available' | 'applying'; latest: string }
    | { s: 'error'; msg: string }
  let upd = $state<Update>({ s: 'idle' })

  async function check(manual: boolean) {
    upd = { s: 'checking' }
    try {
      const info = await UpdateService.Check()
      upd = info.available ? { s: 'available', latest: info.latest } : { s: 'current' }
    } catch (e) {
      // Offline at startup is normal; only a manual check reports the failure.
      upd = manual ? { s: 'error', msg: errText(e) } : { s: 'idle' }
    }
  }

  async function apply(latest: string) {
    upd = { s: 'applying', latest }
    try {
      await UpdateService.Apply()
      await UpdateService.Restart()
    } catch (e) {
      upd = { s: 'error', msg: errText(e) }
    }
  }

  onMount(async () => {
    await reload()
    version = await UpdateService.Version()
    if (version !== 'dev') check(false)
  })
</script>

<div class="desk">
  <nav class="pad" class:dim={app.busy} aria-label="Formularios">
    <p class="pad-name">Utilidades<br />XORCOM</p>
    <p class="pad-sub">Talonario de formularios<br />CompletePBX 5</p>
    <ol class="forms">
      {#each modules as m (m.id)}
        <li>
          <button
            class="form-tab"
            class:on={m.id === current.id}
            aria-current={m.id === current.id ? 'page' : undefined}
            onclick={() => (current = m)}
          >
            <span class="code">{m.code}</span>
            <span class="name">{m.label}</span>
          </button>
        </li>
      {/each}
    </ol>
    <footer class="update" aria-live="polite">
      <span class="ver">{version === 'dev' ? 'Versión de desarrollo' : `Versión ${version}`}</span>
      {#if upd.s === 'checking'}
        <span>Buscando actualizaciones…</span>
      {:else if upd.s === 'current'}
        <span>Al día</span>
      {:else if upd.s === 'available'}
        {@const latest = upd.latest}
        <span>Nueva versión {latest} disponible</span>
        <button class="btn canary small" onclick={() => apply(latest)}>Actualizar y reiniciar</button>
      {:else if upd.s === 'applying'}
        <span>Descargando {upd.latest}…</span>
      {:else if upd.s === 'error'}
        <span class="err">No se pudo actualizar: {upd.msg}</span>
      {/if}
      {#if version !== '' && version !== 'dev' && (upd.s === 'idle' || upd.s === 'current' || upd.s === 'error')}
        <button class="link-light" onclick={() => check(true)}>Buscar actualizaciones</button>
      {/if}
    </footer>
  </nav>

  <main class="sheet">
    <div class="head-wrap">
      <header class="head" class:dim={app.busy}>
        <label class="box pbx">
          <span class="lbl">PBX</span>
          <select
            value={app.active}
            onchange={(e) => setActive(e.currentTarget.value)}
            disabled={app.busy || app.profiles.length === 0}
          >
            {#if app.profiles.length === 0}<option value="">Sin perfiles</option>{/if}
            {#each app.profiles as v (v.profile.id)}
              <option value={v.profile.id}>{v.profile.name}</option>
            {/each}
          </select>
        </label>
        <div class="box"><span class="lbl">Host</span><span class="val">{active?.host ?? '—'}</span></div>
        <div class="box folio">
          <span class="lbl">Folio</span>
          <span class="val">Nº {app.folio ? String(app.folio).padStart(4, '0') : '—'}</span>
        </div>
        <div class="box"><span class="lbl">Fecha</span><span class="val">{today}</span></div>
      </header>
    </div>
    {#key current.id}
      <current.component />
    {/key}
  </main>
</div>

<style>
  .desk { display: grid; grid-template-columns: 232px minmax(0, 1fr); height: 100%; }

  /* A tear-off pad, not a dashboard rail: glued binding strip on top, perforated stubs below. */
  .pad {
    display: flex; flex-direction: column; min-height: 0;
    padding: 20px 0 16px 20px;
    border-top: 10px solid var(--folio); /* the pad's glued edge, in folio red */
    background: var(--ink); color: var(--paper);
  }
  .pad::before { content: ''; display: block; height: 2px; margin: -20px 0 18px -20px; background: var(--paper); }
  .forms li { border-bottom: 1px dashed rgb(255 255 255 / 0.28); }
  .forms li:first-child { border-top: 1px dashed rgb(255 255 255 / 0.28); }
  .pad-name { font: 700 26px/0.95 var(--f-label); letter-spacing: 0.02em; text-transform: uppercase; }
  .pad-sub {
    margin-top: 10px; padding-right: 20px;
    font: 500 12px/1.3 var(--f-label); letter-spacing: 0.06em; text-transform: uppercase;
    color: #c5cde0;
  }
  .forms { flex: 1; margin: 28px 0 0; padding: 0; list-style: none; }
  .form-tab {
    display: flex; align-items: baseline; gap: 12px; width: 100%;
    padding: 10px 16px 10px 12px;
    border: 0; border-radius: 2px 0 0 2px;
    background: none; color: var(--paper); text-align: left; cursor: pointer;
  }
  .form-tab:hover { background: rgb(255 255 255 / 0.08); }
  /* The pulled sheet: the active tab is paper and runs into the page. */
  .form-tab.on { background: var(--paper); color: var(--ink); }
  .code { font: 600 13px/1 var(--f-label); letter-spacing: 0.06em; }
  .name { font: 500 15px/1.2 var(--f-data); }

  .update { display: grid; gap: 6px; padding-right: 20px; font-size: 12px; line-height: 1.35; }
  .ver { font: 600 11px/1 var(--f-label); letter-spacing: 0.09em; text-transform: uppercase; color: #c5cde0; }
  .err { color: var(--pink); }
  .update .btn { justify-self: start; }
  .link-light {
    justify-self: start; padding: 0; border: 0; background: none; cursor: pointer;
    color: var(--paper); font-size: 12px; text-decoration: underline; text-underline-offset: 3px;
  }

  .sheet { overflow-y: auto; min-width: 0; }
  /* Paper margin under the block: scrolled content fades into paper, never against the rule. */
  .head-wrap { position: sticky; top: 0; z-index: 2; padding: 18px 32px 12px; background: var(--paper); }
  .head {
    display: grid;
    grid-template-columns: minmax(200px, 2fr) minmax(150px, 1.4fr) minmax(110px, 0.8fr) minmax(110px, 0.8fr);
    border: 2px solid var(--ink);
  }
  .box { display: grid; gap: 6px; min-width: 0; padding: 8px 12px 10px; border-left: 1px solid var(--ink); }
  .box:first-child { border-left: 0; }
  .val { overflow: hidden; font: 500 17px/1.2 var(--f-data); white-space: nowrap; text-overflow: ellipsis; }
  .folio .val { color: var(--folio); font-weight: 600; }
  .pbx select {
    width: 100%; min-width: 0; padding: 0 22px 0 0;
    border: 0; appearance: none; cursor: pointer;
    background: transparent url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='8'%3E%3Cpath d='M1 1l5 5 5-5' fill='none' stroke='%23233d7a' stroke-width='2'/%3E%3C/svg%3E") right center no-repeat;
    font: 600 17px/1.2 var(--f-data); color: var(--data);
  }
  .pbx select:disabled { background-image: none; cursor: default; }
</style>
