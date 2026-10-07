<script lang="ts">
  import { untrack } from 'svelte'
  import { PinService } from '../../bindings/github.com/skemono/Xorcom-Tools'
  import type { PinApplyResult, PinPreview } from '../../bindings/github.com/skemono/Xorcom-Tools'
  import type { PinList, PinRow } from '../../bindings/github.com/skemono/Xorcom-Tools/pbx'
  import { app, errText } from '../state.svelte'

  // From the lab verification (plan Task 1); see the ledger rulings.
  const NEEDS_PORTAL_APPLY = false
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

  const rows = $derived<PinRow[]>((result?.rows ?? preview?.rows) ?? [])
  // Thousands of lines: let the user see only the ones that need a look.
  const shown = $derived(onlyIssues ? rows.filter((r) => r.status !== 'nuevo' && r.status !== 'aplicado') : rows)
  const canApply = $derived(!!preview && !result && preview.errors === 0 && preview.new > 0 && !app.busy)
  const sepName = (s: string) => (s === 'tab' ? 'tabulador' : `«${s}»`)

  async function loadLists() {
    listsError = ''
    discard()
    if (!app.active) {
      lists = []
      return
    }
    app.busy = true
    try {
      lists = (await PinService.Lists()) ?? []
      if (!lists.some((l) => l.id === listID)) listID = lists[0]?.id ?? 0
    } catch (e) {
      lists = []
      listsError = errText(e)
    } finally {
      app.busy = false
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
    }
  }

  async function runPreview() {
    if (!listID || !path.trim()) return
    app.busy = true
    note = ''
    result = null
    preview = null
    try {
      preview = await PinService.Preview(listID, path.trim(), includeEmpty)
      onlyIssues = preview.errors > 0
    } catch (e) {
      note = errText(e)
    } finally {
      app.busy = false
    }
  }

  async function apply() {
    if (!canApply) return
    app.busy = true
    note = ''
    try {
      result = await PinService.Apply(listID)
      app.folio = result.folio
      lists = (await PinService.Lists()) ?? lists // refreshed counts
    } catch (e) {
      note = errText(e)
    } finally {
      app.busy = false
    }
  }

  function discard() {
    preview = null
    result = null
    note = ''
    portalMsg = ''
    confirmReload = false
  }

  // The portal's Apply reloads the whole PBX with every pending portal change: two steps, like Eliminar.
  async function reloadPBX() {
    if (!confirmReload) {
      confirmReload = true
      setTimeout(() => (confirmReload = false), 4000)
      return
    }
    confirmReload = false
    app.busy = true
    try {
      portalMsg = await PinService.ApplyPortal()
    } catch (e) {
      portalMsg = errText(e)
    } finally {
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
  <div class="title"><span class="code">F-02</span><h1>PINes masivos</h1></div>
  <p class="instr">Cargue PINes desde un CSV. Nada se escribe en la PBX hasta Aplicar.</p>

  <h2 class="lbl section">Lista de PIN</h2>
  {#if listsError}
    <p class="notice">{listsError}</p>
  {:else if app.active && lists.length === 0 && !app.busy}
    <p class="notice">La PBX activa no tiene listas de PIN. Créela en el portal (con al menos un PIN) y presione Actualizar.</p>
  {/if}
  <div class="sheetrow">
    <label class="field grow">
      <span class="lbl">Lista</span>
      <select bind:value={listID} onchange={discard} disabled={app.busy || lists.length === 0}>
        {#each lists as l (l.id)}
          <option value={l.id}>Nº {l.id} · {l.description} ({l.entries} PIN{l.entries === 1 ? '' : 'es'})</option>
        {/each}
      </select>
    </label>
    <div class="field act"><button class="btn small" onclick={loadLists} disabled={app.busy || !app.active}>Actualizar</button></div>
  </div>
  {#if RESAVE_LOSES_DESCRIPTIONS}
    <p class="hint">No edite esta lista en el portal: al guardarla allí se pierden las descripciones.</p>
  {/if}

  <h2 class="lbl section">Archivo</h2>
  <div class="sheetrow">
    <label class="field grow">
      <span class="lbl">Archivo CSV</span>
      <input bind:value={path} onchange={runPreview} placeholder="C:\…\pines.csv" spellcheck="false" disabled={app.busy} />
    </label>
    <div class="field act">
      <button class="btn small" onclick={pick} disabled={app.busy || !listID}>Elegir…</button>
      <button class="btn small" onclick={runPreview} disabled={app.busy || !listID || !path.trim()}>Vista previa</button>
    </div>
  </div>
  <div class="format">
    <span class="lbl">Formato</span>
    <pre class="sample">PIN;Descripcion
4321;Dr. Jose Perez
5*55;Turno nocturno</pre>
    <p class="hint">
      Un PIN por fila: solo números y *. Separador ; o , · encabezado opcional · también acepta pin_list_id,PIN,descripción.
      Las ñ y tildes se reemplazan (José Peña → Jose Pena).
    </p>
  </div>

  {#if preview}
    <p class="detected">
      Separador {sepName(preview.info.separator)} · {preview.info.encoding} · {preview.info.header ? 'con encabezado' : 'sin encabezado'} · {preview.info.rows} filas
    </p>
    <div class="toggles">
      <label class="check"><input type="checkbox" bind:checked={includeEmpty} onchange={runPreview} disabled={app.busy || !!result} /> Incluir PINes sin descripción</label>
      <label class="check"><input type="checkbox" bind:checked={onlyIssues} /> Solo filas con observaciones</label>
      {#if onlyIssues}<span class="tag">Mostrando {shown.length} de {rows.length} filas</span>{/if}
    </div>
    <div class="copy" class:canary={!result}>
      <div class="strip">
        <span class="lbl">{result ? `Copia aplicada · Folio Nº ${String(result.folio).padStart(4, '0')}` : 'Copia — vista previa'}</span>
        <span class="sum">
          {#if result}
            {result.applied} aplicados · {result.skipped} omitidos · {result.failed} fallidos
          {:else}
            {preview.new} nuevos · {preview.existing} ya existen · {preview.noDesc} sin descripción · {preview.errors} con error
          {/if}
        </span>
      </div>
      <table class="lines">
        <thead>
          <tr>
            <th class="lbl num">Nº</th>
            <th class="cell-state"><span class="sr">Estado</span></th>
            <th class="lbl">PIN</th>
            <th class="lbl">Descripción</th>
            <th class="lbl">Resultado</th>
          </tr>
        </thead>
        <tbody>
          {#each shown as r (r.line)}
            <tr class:bad={r.status === 'error' || r.status === 'fallo'}>
              <td class="num">{String(r.line).padStart(3, '0')}</td>
              <td class="cell-state">
                <span class="mark" class:filled={r.status === 'aplicado'} class:struck={r.status === 'error' || r.status === 'fallo'}></span>
              </td>
              <td class="pin">{r.pin}</td>
              <td>{r.description}{#if r.filtered}&nbsp;<span class="tag">(sin tildes)</span>{/if}</td>
              <td>
                {#if r.status === 'aplicado'}
                  <span class="stamp mini">Aplicado</span>
                {:else if r.status === 'fallo'}
                  <span class="stamp mini bad">Falló</span> <span class="why">{r.error}</span>
                {:else if r.status === 'omitido'}
                  <span class="quiet">Omitido</span> <span class="tag">{r.error}</span>
                {:else}
                  <span class:why={r.status === 'error'}>{planned(r)}</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    {#if result && NEEDS_PORTAL_APPLY && result.applied > 0}
      <p class="notice canary">
        Falta aplicar cambios en la PBX para que los PINes funcionen. Esto recarga la PBX y aplica también cualquier otro cambio pendiente del portal.
        <button class="btn small" onclick={reloadPBX} disabled={app.busy}>{confirmReload ? 'Confirmar: recargar la PBX' : 'Aplicar cambios en la PBX'}</button>
      </p>
      {#if portalMsg}<p class="detected">{portalMsg}</p>{/if}
    {/if}
  {/if}

  <div class="actions">
    <button class="btn primary" onclick={apply} disabled={!canApply}>
      {preview && !result ? `Aplicar ${preview.new} PIN${preview.new === 1 ? '' : 'es'}` : 'Aplicar'}
    </button>
    {#if preview}
      <button class="btn" onclick={discard} disabled={app.busy}>{result ? 'Nueva carga' : 'Descartar'}</button>
    {/if}
    <span class="note" role="status">
      {note || (preview && !result && preview.errors > 0 ? 'Corrija el archivo y vuelva a cargarlo: hay filas con error.' : '')}
    </span>
  </div>
</div>

<style>
  .page { padding: 14px 32px 0; }
  .title { display: flex; align-items: center; gap: 14px; }
  .title .code { padding: 5px 8px 4px; border: 2px solid var(--ink); font: 700 15px/1 var(--f-label); letter-spacing: 0.06em; color: var(--ink); }
  h1 { font: 700 30px/1 var(--f-label); letter-spacing: 0.03em; text-transform: uppercase; color: var(--ink); }
  .instr { max-width: 72ch; margin: 6px 0 12px; color: var(--ink-2); font-size: 14px; }
  .section { display: block; margin: 16px 0 6px; }
  .notice { margin-bottom: 8px; padding: 10px 14px; border: 1px solid var(--fail); background: var(--pink); }
  .notice.canary { border-color: var(--ink); background: var(--canary); }
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }

  .sheetrow { display: flex; border-top: 2px solid var(--ink); border-left: 1px solid var(--ink); }
  .field { display: grid; gap: 4px; padding: 6px 12px 7px; border-right: 1px solid var(--ink); border-bottom: 1px solid var(--ink); min-width: 0; }
  .field.grow { flex: 1; }
  .field.act { display: flex; align-items: center; gap: 8px; }
  .field input, .field select { width: 100%; min-width: 0; padding: 0; border: 0; background: transparent; font: 500 16px/1.3 var(--f-data); color: var(--data); }
  .field input::placeholder { color: var(--ink-2); font-style: italic; font-weight: 400; }
  .hint { margin: 6px 0 0; color: var(--ink-2); font-size: 12px; line-height: 1.35; }

  .format { display: grid; grid-template-columns: auto 1fr; gap: 4px 16px; align-items: start; margin-top: 10px; }
  .format .lbl { grid-row: span 2; padding-top: 2px; }
  .sample { margin: 0; font: 400 12px/1.45 var(--f-mono); color: var(--data); }
  .format .hint { margin: 0; }
  .detected { margin: 14px 0 6px; color: var(--ink-2); font-size: 14px; }
  .toggles { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 20px; margin: 0 0 8px; }
  .check { display: inline-flex; align-items: center; gap: 6px; color: var(--ink); font-size: 13px; }
  .check input { width: 16px; height: 16px; margin: 0; accent-color: var(--ink); }

  .copy { border: 2px solid var(--ink); background: var(--paper); }
  .copy.canary { background: var(--canary); }
  .strip { display: flex; justify-content: space-between; align-items: baseline; padding: 8px 12px; border-bottom: 2px solid var(--ink); }
  .sum { font: 600 12px/1 var(--f-label); letter-spacing: 0.06em; text-transform: uppercase; color: var(--ink); }
  .lines { width: 100%; border-collapse: collapse; }
  .lines th { padding: 6px 10px 5px; border-bottom: 1px solid var(--ink); text-align: left; }
  .lines td { padding: 6px 10px; border-bottom: 1px solid rgb(35 61 122 / 0.22); vertical-align: middle; }
  .lines tr.bad td { background: var(--pink); }
  .num { width: 56px; color: var(--ink-2); font-weight: 500; }
  .cell-state { width: 34px; }
  .pin { font-weight: 600; letter-spacing: 0.02em; }
  .tag { color: var(--ink-2); font-size: 12px; }
  .why { color: var(--fail); font-size: 13px; }
  .quiet { font: 600 12px/1 var(--f-label); letter-spacing: 0.09em; text-transform: uppercase; color: var(--ink-2); }

  .actions {
    position: sticky; bottom: 0; z-index: 1;
    display: flex; flex-wrap: wrap; align-items: center; gap: 10px;
    margin-top: 20px; padding: 12px 0 16px;
    border-top: 2px solid var(--ink); background: var(--paper);
  }
  .note { color: var(--ink-2); font-size: 14px; }
</style>
