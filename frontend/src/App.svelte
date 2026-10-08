<script lang="ts">
  import { onMount } from 'svelte'
  import { UpdateService } from '../bindings/github.com/skemono/Xorcom-Tools'
  import Icon from './Icon.svelte'
  import { modules } from './modules'
  import { app, reload, setActive, errText } from './state.svelte'

  let current = $state(modules[0])
  // Height of the sticky PBX sign, for tools that pin their own headings under it.
  let headH = $state(0)
  const active = $derived(app.profiles.find((v) => v.profile.id === app.active)?.profile)
  const channels = $derived(
    active ? [['API', active.api.enabled], ['SSH', active.ssh.enabled], ['AMI', active.ami.enabled]] as const : [],
  )

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

<div class="app">
  <nav class="directory" class:dim={app.busy} aria-label="Herramientas">
    <p class="brand">Utilidades XORCOM<span>CompletePBX 5</span></p>
    <ul class="tools">
      {#each modules as m (m.id)}
        <li>
          <button class="tool" class:on={m.id === current.id} aria-current={m.id === current.id ? 'page' : undefined} onclick={() => (current = m)}>
            <span class="pict"><Icon name={m.icon} /></span>
            <span class="tool-name">{m.label}</span>
            <Icon name="chevron-right" size={18} />
          </button>
        </li>
      {/each}
    </ul>
    <footer class="update" aria-live="polite">
      <span class="ver">{version === 'dev' ? 'Versión de desarrollo' : `Versión ${version}`}</span>
      {#if upd.s === 'checking'}
        <span>Buscando actualizaciones…</span>
      {:else if upd.s === 'current'}
        <span>Al día</span>
      {:else if upd.s === 'available'}
        {@const latest = upd.latest}
        <span>Nueva versión {latest} disponible</span>
        <button class="btn lit small" onclick={() => apply(latest)}><Icon name="download" size={18} />Actualizar y reiniciar</button>
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

  <main class="view" style:--head-h="{headH}px">
    <!-- You are here: the PBX every tool acts on, named large enough to read from across the desk. -->
    <!-- Never dimmed: while a tool writes, which PBX it writes to matters most. -->
    <header class="here" bind:offsetHeight={headH}>
      <span class="pict"><Icon name="phone" size={28} /></span>
      <div class="pbx">
        <label class="switch">
          <span class="sr">PBX activa</span>
          <select value={app.active} onchange={(e) => setActive(e.currentTarget.value)} disabled={app.busy || app.profiles.length === 0}>
            {#if app.profiles.length === 0}<option value="">Ninguna PBX registrada</option>{/if}
            {#each app.profiles as v (v.profile.id)}
              <option value={v.profile.id}>{v.profile.name}</option>
            {/each}
          </select>
          {#if app.profiles.length > 1}<Icon name="chevron-down" size={22} />{/if}
        </label>
        <p class="where">
          {#if active}<code>{active.host}</code> · PBX activa{:else}Registre una PBX en Conexiones{/if}
        </p>
      </div>
      {#if active}
        <ul class="chans" aria-label="Canales">
          {#each channels as [name, on] (name)}
            <li class:off={!on}><span class="dot"></span>{name}<span class="sr">{on ? ' habilitado' : ' deshabilitado'}</span></li>
          {/each}
        </ul>
      {/if}
    </header>
    {#key current.id}
      <current.component />
    {/key}
  </main>
</div>

<style>
  .app { display: grid; grid-template-columns: 220px minmax(0, 1fr); height: 100%; }

  /* The directory sign: tools listed like a hospital's floor directory. */
  .directory { display: flex; flex-direction: column; min-height: 0; padding: 22px 0 18px; background: var(--sign-deep); color: var(--on-sign); }
  .directory :focus-visible { outline-color: var(--lit); }
  .brand { padding: 0 20px 20px; font: 800 18px/1.15 var(--f); }
  .brand span { display: block; margin-top: 4px; font-weight: 500; font-size: 13px; color: var(--on-sign-2); }
  .tools { flex: 1; margin: 0; padding: 0; list-style: none; }
  .tool {
    display: flex; align-items: center; gap: 12px; width: 100%;
    padding: 12px 16px 12px 20px; border: 0; border-top: 1px solid var(--sign-line);
    background: none; color: var(--on-sign-2); font-weight: 700; text-align: left; cursor: pointer;
  }
  .tools li:last-child .tool { border-bottom: 1px solid var(--sign-line); }
  .tool:hover { color: var(--on-sign); background: rgb(255 255 255 / 0.06); }
  .pict { display: grid; place-items: center; flex: none; width: 34px; height: 34px; border-radius: 8px; background: var(--sign-line); color: var(--on-sign); }
  .tool-name { flex: 1; }
  /* The lit entry: the tool you are in runs into the page. */
  .tool.on { background: var(--ground); color: var(--ink); }
  .tool.on .pict { background: var(--sign); }

  .update { display: grid; gap: 8px; padding: 0 20px; font-size: 13px; line-height: 1.35; color: var(--on-sign-2); }
  .ver { font-weight: 700; color: var(--on-sign); }
  .err { color: var(--fail-on-sign); }
  .update .btn { justify-self: start; }
  .link-light {
    justify-self: start; padding: 0; border: 0; background: none; cursor: pointer;
    color: var(--on-sign); font-size: 13px; text-decoration: underline; text-underline-offset: 3px;
  }

  .view { overflow-y: auto; min-width: 0; }
  .here {
    position: sticky; top: 0; z-index: 3;
    display: flex; align-items: center; gap: 16px;
    padding: 14px 28px; background: var(--sign); color: var(--on-sign);
  }
  .here :focus-visible { outline-color: var(--lit); }
  .here .pict { width: 50px; height: 50px; border-radius: 10px; background: var(--on-sign); color: var(--sign); }
  .pbx { min-width: 0; }
  .switch { display: flex; align-items: center; gap: 6px; }
  .switch select {
    min-width: 0; max-width: 100%; padding: 0; border: 0; appearance: none; cursor: pointer;
    background: transparent; color: var(--on-sign);
    font: 800 28px/1.1 var(--f); letter-spacing: -0.01em; text-overflow: ellipsis;
  }
  .switch select:disabled { opacity: 1; color: var(--on-sign); cursor: default; }
  .switch option { color: var(--ink); background: var(--panel); font-size: 15px; font-weight: 500; }
  .where { margin-top: 2px; color: var(--on-sign-2); font-size: 14px; }
  .where code { font-size: 13px; color: var(--on-sign); }
  .chans { display: flex; gap: 8px; margin: 0 0 0 auto; padding: 0; list-style: none; }
  .chans li { display: flex; align-items: center; gap: 7px; padding: 6px 12px 6px 10px; border-radius: 99px; background: rgb(255 255 255 / 0.14); font: 700 13px/1 var(--f); letter-spacing: 0.04em; }
  /* Enabled: a filled dot. Disabled: a hollow dot, dimmed (never struck through: that reads as failed). */
  .dot { width: 9px; height: 9px; border: 2px solid currentColor; border-radius: 50%; background: currentColor; }
  .chans li.off { background: none; color: var(--on-sign-2); }
  .chans li.off .dot { background: none; }
</style>
