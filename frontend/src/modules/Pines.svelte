<script lang="ts">
  import { onDestroy, untrack } from 'svelte'
  import { Events } from '@wailsio/runtime'
  import { PinService } from '../../bindings/github.com/skemono/Xorcom-Tools'
  import type { PinApplyResult, PinPreview } from '../../bindings/github.com/skemono/Xorcom-Tools'
  import type { PinList, PinRow } from '../../bindings/github.com/skemono/Xorcom-Tools/pbx'
  import Icon from '../Icon.svelte'
  import { app, errText, notify, talk } from '../state.svelte'

  // From the lab verification (plan Task 1); see the ledger rulings.
  const NEEDS_PORTAL_APPLY = true // unverified (no outbound route on the lab, by user choice): always offer it
  const RESAVE_LOSES_DESCRIPTIONS = false

  let lists = $state<PinList[]>([])
  let listID = $state(0)
  let listsError = $state('')
  let path = $state('')
  let preview = $state<PinPreview | null>(null)
  let result = $state<PinApplyResult | null>(null)
  let note = $state('')
  let includeEmpty = $state(false)
  let onlyIssues = $state(false)
  let confirmReload = $state(false)
  let portalMsg = $state('')
  let otherPending = $state<boolean | null>(null)
  let pendingNote = $state('')

  // While Aplicar runs: PINes written so far (null until the PBX reports), and seconds elapsed.
  let applying = $state(false)
  let reloading = $state(false)
  let done = $state<number | null>(null)
  let secs = $state(0)
  // A read in flight (never locks navigation): the PIN lists, a preview, the pending-changes check.
  let reading = $state<'' | 'lists' | 'preview' | 'pending'>('')
  const idle = $derived(!app.busy && !reading)

  const rows = $derived<PinRow[]>((result?.rows ?? preview?.rows) ?? [])
  // Thousands of lines: let the user see only the ones that need a look (adjusted descriptions included).
  const shown = $derived(onlyIssues ? rows.filter((r) => (r.status !== 'nuevo' && r.status !== 'aplicado') || r.filtered) : rows)
  const canApply = $derived(!!preview && !result && preview.errors === 0 && preview.new > 0 && idle)
  const sepName = (s: string) => (s === 'tab' ? 'tabulador' : `«${s}»`)
  // Thousands grouped with a narrow no-break space, as on printed forms; Spanish plurals.
  const NNBSP = String.fromCharCode(0x202f)
  const fmt = (v: number) => String(v).replace(/\B(?=(\d{3})+$)/g, NNBSP)
  const count = (v: number, one: string, many: string) => `${fmt(v)} ${v === 1 ? one : many}`
  // Line numbers padded to the widest line in the file, so columns stay aligned past 999.
  const lineWidth = $derived(String(rows.reduce((m, r) => Math.max(m, r.line), 0)).length)
  const fileName = $derived(path.trim().split(/[\\/]/).pop() ?? '')
  // Each impression gets its own tilt (DESIGN: no stamp pasted twice).
  const tilts = ['-3deg', '-1.5deg', '-4.5deg']
  // Why Aplicar is not available yet, said next to it.
  const waiting = $derived(
    reading === 'lists' ? 'Leyendo las listas de PIN de la PBX…' : reading === 'preview' ? 'Preparando la vista previa…'
      : result ? '' : !listID ? 'Elija una lista de PIN.' : !preview ? 'Elija un archivo CSV para ver la vista previa.'
      : preview.errors > 0 ? 'Corrija el archivo y vuelva a cargarlo: hay filas con error.'
      : preview.new === 0 ? 'No hay PINes nuevos para aplicar.' : '',
  )
  // The step you are on is lit: 1 list, 2 file, 3 review, 5 reload (4 is the Aplicar button).
  const lit = $derived(!listID ? 1 : !preview ? 2 : !result ? 3 : NEEDS_PORTAL_APPLY && result.applied > 0 && !portalMsg ? 5 : 0)

  // Only the newest list request may land: a PBX switch mid-read must not show the old PBX's lists.
  let listSeq = 0
  onDestroy(() => listSeq++) // a tab you left drops its pending lists quietly (no stale notice)
  async function loadLists(manual = false) {
    const mine = ++listSeq
    listsError = ''
    discard()
    if (!app.active) {
      lists = []
      reading = ''
      return
    }
    reading = 'lists'
    try {
      const got = (await talk(PinService.Lists())) ?? []
      if (mine !== listSeq) return
      lists = got
      if (!lists.some((l) => l.id === listID)) listID = lists[0]?.id ?? 0
      if (manual) notify('ok', `${count(lists.length, 'lista leída', 'listas leídas')} de la PBX.`)
    } catch (e) {
      if (mine !== listSeq) return
      lists = []
      listsError = errText(e)
      notify('fail', `No se pudieron leer las listas de PIN: ${errText(e)}`, 10000)
    } finally {
      if (mine === listSeq) reading = ''
    }
  }

  // A preview never survives a PBX change.
  let shownPBX: string | null = null
  $effect(() => {
    const id = app.active
    if (id !== shownPBX) {
      shownPBX = id
      untrack(() => loadLists())
    }
  })

  async function pick() {
    try {
      const p = await PinService.PickCSV()
      if (p) {
        path = p
        await runPreview()
      }
    } catch (e) {
      note = errText(e)
      notify('fail', `No se pudo abrir el selector de archivos: ${errText(e)}`, 10000)
    }
  }

  async function runPreview() {
    if (!listID || !path.trim()) return
    reading = 'preview'
    note = ''
    result = null
    preview = null
    try {
      preview = await talk(PinService.Preview(listID, path.trim(), includeEmpty))
      onlyIssues = preview.errors > 0
      if (preview.errors > 0) notify('fail', `El archivo tiene ${count(preview.errors, 'fila con error', 'filas con error')}: corríjalo y vuelva a cargarlo.`, 10000)
    } catch (e) {
      note = errText(e)
      notify('fail', `No se pudo preparar la vista previa: ${errText(e)}`, 10000)
    } finally {
      reading = ''
    }
  }

  // A ticking clock shows a long step is still alive; returns the stop function.
  function tick() {
    secs = 0
    const t = setInterval(() => secs++, 1000)
    return () => clearInterval(t)
  }

  async function apply() {
    if (!canApply) return
    app.busy = true
    applying = true
    done = null
    note = ''
    const stop = tick()
    const off = Events.On('pines:avance', (e) => (done = e.data as number))
    try {
      result = await talk(PinService.Apply(listID))
      if (result.failed) notify('fail', `Aplicado con fallas: ${count(result.failed, 'PIN falló', 'PINes fallaron')}. Revise las filas en rojo.`)
      else notify('ok', `${count(result.applied, 'PIN aplicado', 'PINes aplicados')} en la lista Nº ${listID}.`)
      lists = (await talk(PinService.Lists())) ?? lists // refreshed counts
    } catch (e) {
      note = errText(e)
      notify('fail', `No se pudo aplicar: ${errText(e)}`)
    } finally {
      off()
      stop()
      applying = false
      app.busy = false
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

  // The portal's Apply reloads the whole PBX with every pending portal change: two steps, like Eliminar.
  // The first step also checks for changes someone else saved in the portal and never applied.
  async function reloadPBX() {
    if (!confirmReload) {
      reading = 'pending'
      pendingNote = ''
      try {
        otherPending = await talk(PinService.PendingPortalChanges())
      } catch (e) {
        otherPending = null
        pendingNote = `No se pudo verificar si hay otros cambios pendientes: ${errText(e)}`
      } finally {
        reading = ''
      }
      confirmReload = true
      setTimeout(() => (confirmReload = false), 8000) // time to read the warning
      return
    }
    confirmReload = false
    app.busy = true
    reloading = true
    portalMsg = ''
    const stop = tick()
    try {
      portalMsg = await talk(PinService.ApplyPortal())
      notify('ok', `PBX recargada: ${portalMsg}`)
    } catch (e) {
      portalMsg = errText(e)
      notify('fail', `No se pudo recargar la PBX: ${errText(e)}`)
    } finally {
      stop()
      reloading = false
      app.busy = false
    }
  }

  function planned(r: PinRow) {
    if (r.status === 'nuevo') return 'Nuevo'
    if (r.status === 'existe') return 'Ya existe: se omite'
    if (r.status === 'sindesc') return 'Sin descripción: se omite'
    return `Error: ${r.error}`
  }
</script>

<div class="page">
  <h1>PINes masivos</h1>
  <p class="lead">Cargue PINes desde un CSV. Nada se escribe en la PBX hasta Aplicar.</p>

  {#if listsError}
    <p class="notice"><Icon name="alert" /><span>{listsError}</span></p>
  {:else if app.active && lists.length === 0 && !reading}
    <p class="hint empty-lists">La PBX activa no tiene listas de PIN. Créela en el portal y presione Actualizar.</p>
  {/if}

  <div class="inputs">
    <section class="step" class:lit={lit === 1}>
      <h2 class="step-head"><span class="disc">1</span>Lista de PIN{#if listID && reading !== 'lists'}<span class="ok"><Icon name="check" /></span>{/if}
        {#if reading === 'lists'}<span class="aside doing"><span class="spin"></span>Leyendo listas de la PBX…</span>{/if}</h2>
      <div class="row">
        <label class="sr" for="pin-list">Lista de PIN</label>
        <select id="pin-list" class="control" bind:value={listID} onchange={discard} disabled={!idle || lists.length === 0}>
          {#if reading === 'lists' && lists.length === 0}<option value={0}>Cargando listas…</option>{/if}
          {#each lists as l (l.id)}
            <option value={l.id}>Nº {l.id} · {l.description} ({count(l.entries, 'PIN', 'PINes')})</option>
          {/each}
        </select>
        <button class="btn" onclick={() => loadLists(true)} disabled={!idle || !app.active} title="Volver a leer las listas de la PBX">
          {#if reading === 'lists'}<span class="spin"></span>Leyendo…{:else}<Icon name="refresh" size={18} />Actualizar{/if}
        </button>
      </div>
    </section>
    <section class="step" class:lit={lit === 2}>
      <h2 class="step-head"><span class="disc">2</span>Archivo CSV{#if preview}<span class="ok"><Icon name="check" /></span>{/if}
        {#if reading === 'preview'}<span class="aside doing"><span class="spin"></span>Leyendo el archivo y comparando con la PBX…</span>{/if}</h2>
      <div class="row">
        <label class="sr" for="csv-path">Archivo CSV</label>
        <input id="csv-path" class="control mono" bind:value={path} onchange={runPreview} placeholder="C:\…\pines.csv" spellcheck="false" disabled={!idle} />
        <button class="btn" onclick={pick} disabled={!idle || !listID}><Icon name="folder" size={18} />Elegir…</button>
        <button class="btn" onclick={runPreview} disabled={!idle || !listID || !path.trim()}>
          {#if reading === 'preview'}<span class="spin"></span>Leyendo…{:else}Vista previa{/if}
        </button>
      </div>
      {#if preview}
        <p class="hint detected">
          {fileName} · separador {sepName(preview.info.separator)} · {preview.info.encoding} · {preview.info.header ? 'con encabezado' : 'sin encabezado'} · {count(preview.info.rows, 'fila', 'filas')}
        </p>
      {/if}
    </section>
  </div>
  {#if RESAVE_LOSES_DESCRIPTIONS}
    <p class="hint">No edite esta lista en el portal: al guardarla allí se pierden las descripciones.</p>
  {/if}

  {#if !preview || preview.errors > 0}
    <section class="format">
      <h2 class="format-head">Formato del archivo</h2>
      <pre class="sample">PIN;Descripcion
4321;Dr. Jose Perez
5*55;Turno nocturno</pre>
      <p class="hint">
        Un PIN por fila: solo números y *. Separador ; o , · encabezado opcional · también acepta pin_list_id,PIN,descripción.
        La descripción solo admite letras, números, espacios, guion y guion bajo (regla de la PBX): ñ, tildes y otros signos se ajustan solos (José Peña; Dr. → Jose Pena Dr).
      </p>
    </section>
  {/if}

  {#if preview}
    <section class="review" class:lit={!result}>
      <div class="bar">
        {#if result}
          <span class="disc done"><Icon name="check" size={16} /></span>
          <h2 class="bar-title">Resultado</h2>
          <span class="stamp" class:bad={result.failed > 0} style:--r="-2deg">{result.failed ? 'Con fallas' : 'Aplicado'}</span>
          <span class="sum">{count(result.applied, 'aplicado', 'aplicados')} · {count(result.skipped, 'omitido', 'omitidos')} · <span class:bad={result.failed > 0}>{count(result.failed, 'fallido', 'fallidos')}</span></span>
        {:else if applying}
          <span class="disc">4</span>
          <h2 class="bar-title">Aplicando</h2>
          <span class="sum" role="status">
            {done === null ? 'Conectando con la PBX…' : done < preview.new ? `Escribiendo ${fmt(done)} de ${count(preview.new, 'PIN', 'PINes')}` : 'Verificando…'} · {secs} s
          </span>
          <progress class="progress" max={preview.new} value={done ?? 0} aria-label="Avance"></progress>
        {:else}
          <span class="disc">3</span>
          <h2 class="bar-title">Revise la vista previa</h2>
          <Icon name="arrow-right" />
          <span class="sum">{count(preview.new, 'nuevo', 'nuevos')} · {count(preview.existing, 'ya existe', 'ya existen')} · {fmt(preview.noDesc)} sin descripción · <span class:bad={preview.errors > 0}>{fmt(preview.errors)} con error</span></span>
        {/if}
      </div>
      <div class="toggles">
        <label class="check"><input type="checkbox" bind:checked={includeEmpty} onchange={runPreview} disabled={!idle || !!result} /> Incluir PINes sin descripción</label>
        <label class="check"><input type="checkbox" bind:checked={onlyIssues} /> Solo filas a revisar</label>
        {#if onlyIssues}<span class="tag">Mostrando {fmt(shown.length)} de {count(rows.length, 'fila', 'filas')}</span>{/if}
      </div>
      <table class="lines">
        <colgroup><col class="c-num" /><col class="c-state" /><col class="c-pin" /><col /><col class="c-res" /></colgroup>
        <thead>
          <tr>
            <th class="num">Nº</th>
            <th><span class="sr">Estado</span></th>
            <th>PIN</th>
            <th>Descripción</th>
            <th>Resultado</th>
          </tr>
        </thead>
        <tbody>
          {#each shown as r (r.line)}
            <tr class:bad={r.status === 'error' || r.status === 'fallo'}>
              <td class="num">{String(r.line).padStart(Math.max(2, lineWidth), '0')}</td>
              <td><span class="mark" class:ok={r.status === 'aplicado'} class:bad={r.status === 'error' || r.status === 'fallo'} class:skip={r.status === 'existe' || r.status === 'sindesc' || r.status === 'omitido'}></span></td>
              <td class="pin">{r.pin}</td>
              <td>{r.description}{#if r.filtered}&nbsp;<span class="tag" title="Se quitaron ñ, tildes o signos que la PBX no acepta">(ajustada)</span>{/if}</td>
              <td>
                {#if r.status === 'aplicado'}
                  <span class="stamp mini" style:--r={tilts[r.line % 3]}>Aplicado</span>
                {:else if r.status === 'fallo'}
                  <span class="stamp mini bad" style:--r={tilts[r.line % 3]}>Falló</span> <span class="why">{r.error}</span>
                {:else if r.status === 'omitido'}
                  <span class="quiet">Omitido · {r.error}</span>
                {:else if r.status === 'existe' || r.status === 'sindesc'}
                  <span class="quiet">{r.status === 'existe' ? 'Ya existe' : 'Sin descripción'} · se omite</span>
                {:else}
                  <span class:why={r.status === 'error'} class:new={r.status === 'nuevo'}>{planned(r)}</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </section>

    {#if result && NEEDS_PORTAL_APPLY && result.applied > 0}
      <section class="step reload" class:lit={lit === 5}>
        <h2 class="step-head"><span class="disc">5</span>Recargue la PBX{#if portalMsg && !reloading}<span class="ok"><Icon name="check" /></span>{/if}</h2>
        <p class="reload-why">Para asegurar que la PBX use los PINes nuevos, aplique los cambios. Esto recarga la PBX y aplica también cualquier otro cambio pendiente del portal.</p>
        <div class="reload-row">
          <!-- Two-step: the second click of a double-click (detail 2) must never confirm. -->
          <button class="btn" class:confirm={confirmReload} onclick={(e) => e.detail <= 1 && reloadPBX()} disabled={!idle}>
            {#if reading === 'pending'}<span class="spin"></span>Verificando cambios pendientes…{:else if reloading}<span class="spin"></span>Recargando la PBX…{:else}<Icon name="reload" size={18} />{confirmReload ? 'Confirmar: recargar la PBX' : 'Aplicar cambios en la PBX'}{/if}
          </button>
          {#if reloading}
            <span class="tag" role="status">Recargando la PBX… {secs} s</span>
          {:else if confirmReload && otherPending}
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

  <div class="actions">
    <button class="btn primary" onclick={apply} disabled={!canApply}>
      {#if applying}<span class="spin"></span>Aplicando…{:else}<span class="disc">4</span>{preview && !result ? `Aplicar ${count(preview.new, 'PIN', 'PINes')}` : 'Aplicar'}{/if}
    </button>
    {#if preview}
      <button class="btn" onclick={discard} disabled={!idle}>{result ? 'Nueva carga' : 'Descartar'}</button>
    {/if}
    <span class="note" role="status">{note || waiting}</span>
  </div>
</div>

<style>
  .page { padding: 22px 28px 0; }
  .notice, .empty-lists { margin-bottom: 14px; }
  .ok { display: inline-flex; color: var(--ok); }

  /* Steps 1 and 2 stack, so the list name and the path get the full width; side by side only on wide windows. */
  .inputs { display: grid; gap: 14px; align-items: start; }
  @media (min-width: 1500px) { .inputs { grid-template-columns: minmax(0, 1fr) minmax(0, 1.5fr); } }
  .row { display: flex; gap: 8px; }
  .row .control { flex: 1; }
  .detected { margin-top: 8px; }

  .format { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 6px 18px; align-items: start; margin-top: 14px; padding: 12px 16px; border: 2px dashed var(--rule-strong); border-radius: 12px; }
  .format-head { grid-column: 1 / -1; font: 800 15px/1.2 var(--f); }
  .sample { margin: 0; padding: 8px 12px; border-radius: 8px; background: var(--panel); font: 400 13px/1.5 var(--f-mono); }

  /* Step 3: the preview, lit yellow while it is the step you are on. */
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
  .toggles { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 22px; padding: 10px 16px; border-bottom: 1px solid var(--rule); }
  .review.lit .toggles { border-bottom-color: var(--lit-rule); }

  /* Fixed layout: columns never reflow when the filter or the stamps change a row. */
  .lines { width: 100%; table-layout: fixed; border-collapse: separate; border-spacing: 0; }
  .c-num { width: 76px; }
  .c-state { width: 40px; }
  .c-pin { width: 130px; }
  .c-res { width: 40%; }
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
  .pin { font-weight: 800; letter-spacing: 0.02em; }
  .tag { color: var(--mute); font-size: 13px; }
  .new { font-weight: 700; }
  .why { color: var(--fail); font-size: 14px; text-wrap: pretty; }
  .quiet { color: var(--mute); }
  /* On the yellow preview, secondary text is tinted from the yellow, never grey; on a failed row it is ink. */
  .review.lit :is(.num, .tag, .quiet), .review.lit .lines th, .review.lit .toggles .tag { color: var(--lit-mute); }
  .lines tr.bad :is(.num, .tag, .quiet) { color: var(--ink); }
  /* State marks: hollow pending, green applied, red failed, dashed skipped. */
  .mark { display: block; width: 14px; height: 14px; border: 2px solid var(--rule-strong); border-radius: 50%; }
  .mark.ok { border-color: var(--ok); background: var(--ok); }
  .mark.bad { border-color: var(--fail); background: var(--fail); }
  .mark.skip { border-style: dashed; border-color: var(--mute); }

  .reload { margin-top: 14px; }
  .step.lit { border-color: var(--ink); }
  .step.lit .disc { background: var(--lit); color: var(--ink); }
  .reload-why { max-width: 72ch; margin-bottom: 12px; }
  .reload-row { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; }
  .btn.confirm { border-color: var(--fail); color: var(--fail); }
  .warn { display: inline-flex; align-items: center; gap: 6px; color: var(--fail); font-weight: 700; }
  .portal-msg { margin-top: 10px; color: var(--mute); }
</style>
