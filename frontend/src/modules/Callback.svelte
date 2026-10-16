<script lang="ts">
  import { onDestroy, untrack } from 'svelte'
  import { Events } from '@wailsio/runtime'
  import { CallbackService, ProfileService } from '../../bindings/github.com/skemono/Xorcom-Tools'
  import type { CallbackPreview, CallbackResult, CallbackView } from '../../bindings/github.com/skemono/Xorcom-Tools'
  import type { CallbackConfig, CallbackQueueView, CallbackStep } from '../../bindings/github.com/skemono/Xorcom-Tools/pbx'
  import Icon from '../Icon.svelte'
  import { app, errText, notify, talk } from '../state.svelte'

  const KEYS = ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '*', '#']

  let view = $state<CallbackView | null>(null)
  let viewError = $state('')
  let cfg = $state<CallbackConfig | null>(null)
  let picked = $state<string[]>([])
  let testNumber = $state('')
  let dropWaiting = $state(false)
  let mode = $state<'install' | 'remove'>('install')
  let preview = $state<CallbackPreview | null>(null)
  let result = $state<CallbackResult | null>(null)
  let note = $state('')
  let applying = $state(false)
  let done = $state(0)
  let reading = $state<'' | 'state' | 'preview' | 'pending'>('')
  let confirmReload = $state(false)
  let reloading = $state(false)
  let portalMsg = $state('')
  let otherPending = $state<boolean | null>(null)
  let pendingNote = $state('')
  const idle = $derived(!app.busy && !reading)

  const st = $derived(view?.state)
  // Which queues can take or drop callback comes from Go (CallbackQueueView): the sheet only shows it.
  const queues = $derived<CallbackQueueView[]>(view?.queues ?? [])
  const usable = (v: CallbackQueueView) => (mode === 'install' ? !v.foreign : v.hooked)
  const steps = $derived<CallbackStep[]>((result?.steps ?? preview?.plan.steps) ?? [])
  const canApply = $derived(!!preview && !result && preview.blocked === 0 && preview.new > 0 && idle)
  const verb = $derived(mode === 'install' ? 'Aplicar' : 'Quitar')
  const waiting = $derived(
    reading === 'state' ? 'Leyendo la PBX…' : reading === 'preview' ? 'Preparando la vista previa…'
      : result ? '' : !st ? '' : !st.hasCC ? '' : picked.length === 0 ? 'Elija al menos una cola.'
      : mode === 'install' && !/^\d{3,20}$/.test(testNumber) ? 'Escriba un número de prueba para revisar la ruta de salida.'
      : !preview ? 'Genere la vista previa.' : preview.blocked > 0 ? 'Hay pasos bloqueados: quite esas colas.'
      : preview.new === 0 ? 'No hay cambios: todo ya está así.' : '',
  )
  const fileState = $derived(!st ? '' : !st.fileExists ? 'no existe'
    : !view?.installed ? 'distinto: editado a mano (se reemplaza al aplicar)'
    : st.contexts === 3 ? 'instalado' : 'instalado, contextos sin cargar (se recarga al aplicar)')
  // Estado shows what the PBX has, never the editable settings.
  const CB_APP = 'create_callback,s,1'
  const foundApp = $derived(st?.apps?.find((a) => a.custom === CB_APP))
  const appState = $derived(foundApp ? `código ${foundApp.extension} «${foundApp.description}»` : 'no existe')
  const ivrState = $derived.by(() => {
    const ivr = foundApp && st?.ivrs?.find((i) => i.entries?.length === 1 && i.entries[0].destKind === 'custom_application' && i.entries[0].destIndex === foundApp.id)
    return ivr ? `«${ivr.description}» (Nº ${ivr.id}) · tecla ${ivr.entries![0].option}` : 'no existe'
  })
  const lit = $derived(!st?.hasCC ? 0 : picked.length === 0 ? 3 : !preview ? 4 : !result ? 5 : result.applied > 0 && !portalMsg ? 6 : 0)
  const tilts = ['-3deg', '-1.5deg', '-4.5deg']

  // Only the newest read may land: a PBX switch mid-read must not show the old PBX.
  let seq = 0
  onDestroy(() => seq++)
  async function load(manual = false) {
    const mine = ++seq
    discard()
    viewError = ''
    if (!app.active) {
      view = null
      reading = ''
      return
    }
    reading = 'state'
    try {
      const v = await talk(CallbackService.State())
      if (mine !== seq) return
      view = v
      cfg = { ...v.config }
      picked = []
      if (manual) notify('ok', 'Estado leído de la PBX.')
    } catch (e) {
      if (mine !== seq) return
      viewError = errText(e)
      notify('fail', `No se pudo leer la PBX: ${errText(e)}`, 10000)
    } finally {
      if (mine === seq) reading = ''
    }
  }

  // A preview never survives a PBX change.
  let shownPBX: string | null = null
  $effect(() => {
    const id = app.active
    if (id !== shownPBX) {
      shownPBX = id
      view = null // a different PBX never shows the old one's state
      untrack(() => load())
    }
  })

  async function runPreview() {
    if (!cfg || picked.length === 0) return
    reading = 'preview'
    note = ''
    preview = null
    result = null
    try {
      preview = await talk(mode === 'install'
        ? CallbackService.PreviewInstall(cfg, picked, testNumber.trim())
        : CallbackService.PreviewRemove(picked, dropWaiting))
      if (preview.blocked) notify('fail', 'Hay pasos bloqueados en la vista previa: revise las colas marcadas en rojo.', 10000)
    } catch (e) {
      note = errText(e)
      notify('fail', `No se pudo preparar la vista previa: ${errText(e)}`, 10000)
    } finally {
      reading = ''
    }
  }

  async function apply() {
    if (!canApply) return
    const mine = seq
    app.busy = true
    applying = true
    done = 0
    note = ''
    const off = Events.On('callback:avance', (e) => (done = e.data as number))
    try {
      result = await talk(CallbackService.Apply())
      if (result.failed) notify('fail', `Folio ${result.folio}: ${result.failed} paso(s) fallaron. Revise las líneas en rojo.`)
      else notify('ok', `Folio ${result.folio}: ${result.applied} paso(s) aplicados.`)
    } catch (e) {
      // The Go side claims the preview once: a refused attempt needs a fresh preview.
      preview = null
      note = errText(e)
      notify('fail', `No se pudo aplicar: ${errText(e)}`)
    } finally {
      off()
      applying = false
      app.busy = false
    }
    if (result) {
      try {
        const v = await talk(CallbackService.State())
        if (mine === seq) view = v // a PBX switch meanwhile must not land the old state
      } catch { /* the result stays; Actualizar re-reads */ }
    }
  }

  function discard() {
    preview = null
    result = null
    note = ''
    portalMsg = ''
    confirmReload = false
    otherPending = null
    pendingNote = ''
  }

  function toggle(q: string, on: boolean) {
    picked = on ? [...picked, q] : picked.filter((x) => x !== q)
    discard()
  }

  function setMode(m: 'install' | 'remove') {
    mode = m
    picked = []
    discard()
  }

  // Same two-step portal Apply as F-02 (it reloads the whole PBX with every pending portal change).
  async function reloadPBX() {
    if (!confirmReload) {
      reading = 'pending'
      pendingNote = ''
      try {
        otherPending = await talk(ProfileService.PendingPortalChanges())
      } catch (e) {
        otherPending = null
        pendingNote = `No se pudo verificar si hay otros cambios pendientes: ${errText(e)}`
      } finally {
        reading = ''
      }
      confirmReload = true
      setTimeout(() => (confirmReload = false), 8000)
      return
    }
    confirmReload = false
    app.busy = true
    reloading = true
    portalMsg = ''
    try {
      portalMsg = await talk(ProfileService.ApplyPortal())
      notify('ok', `PBX recargada: ${portalMsg}`)
    } catch (e) {
      portalMsg = errText(e)
      notify('fail', `No se pudo recargar la PBX: ${errText(e)}`)
    } finally {
      reloading = false
      app.busy = false
    }
  }

  const failedRow = (s: CallbackStep) => s.status === 'bloqueado' || s.status === 'fallo'
  const planned = (s: CallbackStep) => (s.status === 'omitir' ? 'Ya está así: se omite' : s.status === 'bloqueado' ? `Bloqueado: ${s.error}` : s.after)
  const showFile = $derived(!!preview?.plan.dialplan)
</script>

<div class="page">
  <h1>Devolución de llamada</h1>
  <p class="lead">Quien espera en una cola marca una tecla y la PBX le devuelve la llamada cuando llega su turno. Nada se escribe en la PBX hasta Aplicar.</p>

  {#if viewError}
    <p class="notice"><Icon name="alert" /><span>{viewError}</span></p>
  {:else if !app.active}
    <p class="hint">Elija una PBX en el encabezado.</p>
  {/if}

  {#if st && !st.hasCC}
    <p class="notice"><Icon name="alert" />Esta PBX no tiene Call Center: la devolución de llamada no está disponible.</p>
  {:else if st && cfg}
    <section class="step">
      <h2 class="step-head"><span class="disc">1</span>Estado{#if reading === 'state'}<span class="aside doing"><span class="spin"></span>Leyendo la PBX…</span>{/if}</h2>
      <dl class="state">
        <dt>Archivo</dt><dd>{fileState}</dd>
        <dt>Registro</dt><dd>{st.logOwner || 'sin dueño'}</dd>
        <dt>Aplicación</dt><dd>{appState}</dd>
        <dt>IVR</dt><dd>{ivrState}</dd>
        <dt>En espera</dt><dd>{st.waiting?.length ?? 0}</dd>
      </dl>
      <button class="btn" onclick={() => load(true)} disabled={!idle || !app.active} title="Volver a leer el estado de la PBX">
        {#if reading === 'state'}<span class="spin"></span>Leyendo…{:else}<Icon name="refresh" size={18} />Actualizar{/if}
      </button>
    </section>

    <section class="step">
      <h2 class="step-head"><span class="disc">2</span>Qué hacer</h2>
      <div class="row">
        <label class="check"><input type="radio" name="cb-mode" checked={mode === 'install'} onchange={() => setMode('install')} disabled={!idle} /> Instalar</label>
        <label class="check"><input type="radio" name="cb-mode" checked={mode === 'remove'} onchange={() => setMode('remove')} disabled={!idle} /> Quitar</label>
      </div>
    </section>

    <section class="step" class:lit={lit === 3}>
      <h2 class="step-head"><span class="disc">3</span>Colas{#if picked.length}<span class="ok"><Icon name="check" /></span>{/if}</h2>
      {#if queues.length === 0}<p class="hint">Esta PBX no tiene colas.</p>{/if}
      <ul class="queues">
        {#each queues as v (v.queue.extension)}
          <li>
            <label class="check">
              <input type="checkbox" checked={picked.includes(v.queue.extension)} onchange={(e) => toggle(v.queue.extension, e.currentTarget.checked)} disabled={!idle || !usable(v)} />
              Nº {v.queue.extension} · {v.queue.description}
            </label>
            {#if v.attached}<span class="tag">con devolución</span>{/if}
            {#if v.foreign && mode === 'install'}<span class="why">{v.foreign}</span>{/if}
            {#if v.waiting}<span class="tag">{v.waiting} pendiente(s)</span>{/if}
          </li>
        {/each}
      </ul>
    </section>

    <section class="step">
      <h2 class="step-head"><span class="disc">4</span>Ajustes</h2>
      {#if mode === 'install'}
        <div class="settings" onchange={discard} role="group" aria-label="Ajustes">
          <label for="cb-key">Tecla</label>
          <select id="cb-key" class="control" bind:value={cfg.key} disabled={!idle}>{#each KEYS as k}<option value={k}>{k}</option>{/each}</select>
          <label for="cb-app">Código de la aplicación</label>
          <input id="cb-app" class="control mono" bind:value={cfg.appCode} disabled={!idle} spellcheck="false" />
          <label for="cb-prio">Prioridad</label>
          <input id="cb-prio" class="control" type="number" min="0" bind:value={cfg.priority} disabled={!idle} />
          <label for="cb-retries">Reintentos</label>
          <input id="cb-retries" class="control" type="number" min="0" bind:value={cfg.retries} disabled={!idle} />
          <label for="cb-mins">Minutos entre reintentos</label>
          <input id="cb-mins" class="control" type="number" min="0" value={cfg.retrySecs / 60} oninput={(e) => { if (cfg) cfg.retrySecs = Math.round(e.currentTarget.valueAsNumber * 60) || 0 }} disabled={!idle} />
          <label for="cb-ring">Segundos de timbre</label>
          <input id="cb-ring" class="control" type="number" min="1" bind:value={cfg.ringSecs} disabled={!idle} />
          <label for="cb-name">Nombre para el agente</label>
          <input id="cb-name" class="control" bind:value={cfg.callerName} disabled={!idle} />
          <label for="cb-prefix">Prefijo</label>
          <input id="cb-prefix" class="control mono" bind:value={cfg.prefix} disabled={!idle} spellcheck="false" />
          <label for="cb-rec">Grabación de confirmación</label>
          <select id="cb-rec" class="control" bind:value={cfg.recording} disabled={!idle}>
            <option value={0}>Ninguna</option>
            {#each st.recordings ?? [] as r (r.id)}<option value={r.id}>{r.name}</option>{/each}
          </select>
          <label for="cb-test">Número de prueba</label>
          <div>
            <input id="cb-test" class="control mono" bind:value={testNumber} disabled={!idle} inputmode="numeric" spellcheck="false" />
            <p class="hint">Un número externo real: se revisa que una ruta de salida lo acepte; no se llama</p>
          </div>
        </div>
      {:else}
        <label class="check"><input type="checkbox" bind:checked={dropWaiting} onchange={discard} disabled={!idle} /> Descartar devoluciones pendientes de estas colas</label>
      {/if}
    </section>

    {#if preview}
      <section class="review" class:lit={!result}>
        <div class="bar">
          {#if result}
            <span class="disc done"><Icon name="check" size={16} /></span>
            <h2 class="bar-title">Resultado</h2>
            <span class="stamp" class:bad={result.failed > 0} style:--r="-2deg">{result.failed ? 'Con fallas' : 'Aplicado'}</span>
            <span class="sum">{result.applied} aplicado(s) · {result.skipped} omitido(s) · <span class:bad={result.failed > 0}>{result.failed} fallido(s)</span>{#if result.notRun} · {result.notRun} sin ejecutar{/if}</span>
          {:else if applying}
            <span class="disc">5</span>
            <h2 class="bar-title">Aplicando</h2>
            <span class="sum" role="status">{done} de {steps.length} pasos</span>
            <progress class="progress" max={steps.length} value={done} aria-label="Avance"></progress>
          {:else}
            <span class="disc">5</span>
            <h2 class="bar-title">Revise la vista previa</h2>
            <Icon name="arrow-right" />
            <span class="sum">{preview.new} nuevo(s) · {preview.skip} sin cambio · <span class:bad={preview.blocked > 0}>{preview.blocked} bloqueado(s)</span></span>
          {/if}
        </div>
        {#if preview.route}<p class="route">Ruta de salida: «{preview.route}»</p>{/if}
        <table class="lines">
          <colgroup><col class="c-num" /><col class="c-state" /><col /><col class="c-cur" /><col class="c-res" /></colgroup>
          <thead>
            <tr>
              <th class="num">Nº</th>
              <th><span class="sr">Estado</span></th>
              <th>Paso</th>
              <th>Actual</th>
              <th>{result ? 'Resultado' : 'Nuevo'}</th>
            </tr>
          </thead>
          <tbody>
            {#each steps as s, i (i)}
              <tr class:bad={failedRow(s)}>
                <td class="num">{String(i + 1).padStart(2, '0')}</td>
                <td><span class="mark" class:ok={s.status === 'aplicado'} class:bad={failedRow(s)} class:skip={s.status === 'omitir' || s.status === 'omitido' || s.status === 'no_ejecutado'}></span></td>
                <td>{s.label}</td>
                <td class="quiet">{s.before}</td>
                <td>
                  {#if s.status === 'aplicado'}
                    <span class="stamp mini" style:--r={tilts[i % 3]}>Aplicado</span>
                  {:else if s.status === 'fallo'}
                    <span class="stamp mini bad" style:--r={tilts[i % 3]}>Falló</span> <span class="why">{s.error}</span>
                  {:else if s.status === 'omitido'}
                    <span class="quiet">Omitido{s.error ? ` · ${s.error}` : ''}</span>
                  {:else if s.status === 'no_ejecutado'}
                    <span class="quiet">No se ejecutó</span>
                  {:else if s.status === 'omitir'}
                    <span class="quiet">{planned(s)}</span>
                  {:else}
                    <span class:why={s.status === 'bloqueado'} class:new={s.status === 'nuevo'}>{planned(s)}</span>
                  {/if}
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
        {#if showFile}
          <details class="file">
            <summary>Archivo que se instalará</summary>
            <pre class="sample">{preview.plan.dialplan}</pre>
          </details>
        {/if}
      </section>

      {#if result && result.applied > 0}
        <section class="step reload" class:lit={lit === 6}>
          <h2 class="step-head"><span class="disc">6</span>Recargue la PBX{#if portalMsg && !reloading}<span class="ok"><Icon name="check" /></span>{/if}</h2>
          <p class="reload-why">Para asegurar que la PBX use los cambios, aplíquelos. Esto recarga la PBX y aplica también cualquier otro cambio pendiente del portal.</p>
          {#if mode === 'install'}
            <p class="reload-why">Grabe en el portal el anuncio periódico de la cola: «Presione {cfg.key} para que le devolvamos la llamada».</p>
          {/if}
          <div class="reload-row">
            <!-- Two-step: the second click of a double-click (detail 2) must never confirm. -->
            <button class="btn" class:confirm={confirmReload} onclick={(e) => e.detail <= 1 && reloadPBX()} disabled={!idle}>
              {#if reading === 'pending'}<span class="spin"></span>Verificando cambios pendientes…{:else if reloading}<span class="spin"></span>Recargando la PBX…{:else}<Icon name="reload" size={18} />{confirmReload ? 'Confirmar: recargar la PBX' : 'Aplicar cambios en la PBX'}{/if}
            </button>
            {#if confirmReload && otherPending}
              <span class="warn"><Icon name="alert" size={18} />Hay otros cambios pendientes en el portal; también se aplicarán.</span>
            {:else if confirmReload && otherPending === false}
              <span class="tag">No hay otros cambios pendientes en el portal.</span>
            {:else if confirmReload && pendingNote}
              <span class="tag">{pendingNote}</span>
            {/if}
          </div>
          {#if portalMsg}<p class="portal-msg">{portalMsg}</p>{/if}
        </section>
      {/if}
    {/if}
  {/if}

  <div class="actions">
    <button class="btn primary" onclick={apply} disabled={!canApply}>
      {#if applying}<span class="spin"></span>Aplicando… {done} de {steps.length}{:else}<span class="disc">5</span>{verb}{/if}
    </button>
    <button class="btn" onclick={runPreview} disabled={!idle || !cfg || picked.length === 0 || !st?.hasCC}>
      {#if reading === 'preview'}<span class="spin"></span>Leyendo…{:else}Vista previa{/if}
    </button>
    {#if preview || result}
      <button class="btn" onclick={discard} disabled={!idle}>{result ? 'Nueva carga' : 'Descartar'}</button>
    {/if}
    <span class="note" role="status">{note || waiting}</span>
  </div>
</div>

<style>
  .page { padding: 22px 28px 0; }
  .notice { margin-bottom: 14px; }
  .step { margin-top: 14px; }
  .ok { display: inline-flex; color: var(--ok); }
  .row { display: flex; gap: 22px; }
  .state { display: grid; grid-template-columns: max-content 1fr; gap: 4px 18px; margin: 0 0 12px; }
  .state dt { color: var(--mute); }
  .state dd { margin: 0; font-weight: 700; }
  .queues { list-style: none; margin: 0; padding: 0; display: grid; gap: 8px; }
  .queues li { display: flex; flex-wrap: wrap; align-items: center; gap: 4px 12px; }
  .settings { display: grid; grid-template-columns: minmax(180px, max-content) minmax(0, 360px); gap: 8px 18px; align-items: center; }
  .settings .hint { margin-top: 4px; }

  .review { margin-top: 14px; border: 2px solid var(--rule); border-radius: 12px; background: var(--panel); overflow: clip; }
  .review.lit { border-color: var(--ink); background: var(--lit-soft); }
  .bar {
    position: sticky; top: var(--head-h, 0px); z-index: 2;
    display: flex; align-items: center; gap: 12px;
    height: 54px; padding: 0 16px; border-bottom: 2px solid var(--rule); background: var(--panel);
  }
  .review.lit .bar { border-bottom-color: var(--ink); background: var(--lit); }
  .review.lit .bar .disc { background: var(--ink); color: var(--lit); }
  .bar-title { font: 800 16px/1.2 var(--f); white-space: nowrap; }
  .bar .stamp { padding: 3px 9px 2px; font-size: 13px; }
  .sum { margin-left: auto; font-weight: 700; font-size: 14px; white-space: nowrap; }
  .sum .bad { color: var(--fail); }
  .progress { position: absolute; left: 0; bottom: -2px; width: 100%; height: 5px; border: 0; appearance: none; background: transparent; }
  .progress::-webkit-progress-bar { background: transparent; }
  .progress::-webkit-progress-value { background: var(--sign); }
  .route { padding: 10px 16px; border-bottom: 1px solid var(--rule); font-weight: 700; }
  .review.lit .route { border-bottom-color: var(--lit-rule); }

  /* Fixed layout: columns never reflow when stamps change a row. */
  .lines { width: 100%; table-layout: fixed; border-collapse: separate; border-spacing: 0; }
  .c-num { width: 60px; }
  .c-state { width: 40px; }
  .c-cur { width: 22%; }
  .c-res { width: 34%; }
  .lines th {
    position: sticky; top: calc(var(--head-h, 0px) + 54px); z-index: 1;
    padding: 9px 12px 8px 16px; border-bottom: 1px solid var(--rule); background: var(--panel);
    color: var(--mute); font: 700 13px/1 var(--f); letter-spacing: 0.06em; text-transform: uppercase; text-align: left;
  }
  .review.lit .lines th { border-bottom-color: var(--lit-rule); background: var(--lit-soft); }
  .lines td { padding: 9px 12px 9px 16px; border-bottom: 1px solid var(--rule); vertical-align: middle; overflow-wrap: anywhere; }
  .review.lit .lines td { border-bottom-color: var(--lit-rule); }
  .lines tbody tr:last-child td { border-bottom: 0; }
  .lines tr.bad td { background: var(--fail-soft); }
  .num { color: var(--mute); }
  .tag { color: var(--mute); font-size: 13px; }
  .new { font-weight: 700; }
  .why { color: var(--fail); font-size: 14px; text-wrap: pretty; }
  .quiet { color: var(--mute); }
  .review.lit :is(.num, .tag, .quiet), .review.lit .lines th { color: var(--lit-mute); }
  .lines tr.bad :is(.num, .tag, .quiet) { color: var(--ink); }
  .mark { display: block; width: 14px; height: 14px; border: 2px solid var(--rule-strong); border-radius: 50%; }
  .mark.ok { border-color: var(--ok); background: var(--ok); }
  .mark.bad { border-color: var(--fail); background: var(--fail); }
  .mark.skip { border-style: dashed; border-color: var(--mute); }
  .file { padding: 10px 16px; border-top: 1px solid var(--rule); }
  .file summary { cursor: pointer; font-weight: 700; }
  .sample { margin: 8px 0 0; padding: 8px 12px; border-radius: 8px; background: var(--panel); font: 400 13px/1.5 var(--f-mono); overflow-x: auto; }

  .step.lit { border-color: var(--ink); }
  .step.lit .disc { background: var(--lit); color: var(--ink); }
  .reload-why { max-width: 72ch; margin-bottom: 12px; }
  .reload-row { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; }
  .btn.confirm { border-color: var(--fail); color: var(--fail); }
  .warn { display: inline-flex; align-items: center; gap: 6px; color: var(--fail); font-weight: 700; }
  .portal-msg { margin-top: 10px; color: var(--mute); }
</style>
