<script lang="ts">
  import { tick, untrack } from 'svelte'
  import { ProfileService } from '../../bindings/github.com/skemono/Xorcom-Tools'
  import type { ChannelResult, Profile, Secrets } from '../../bindings/github.com/skemono/Xorcom-Tools/pbx'
  import { app, reload, setActive, errText } from '../state.svelte'

  const blank = (): Profile => ({
    id: '', name: '', host: '',
    api: { enabled: true, baseURL: '', user: 'admin', certSHA256: '' },
    ssh: { enabled: true, port: 22, user: 'root', keyPath: '', hostKeySHA256: '' },
    ami: { enabled: true, port: 5038, user: '' },
  })
  const noSecrets = (): Secrets => ({ api: '', ssh: '', sshKey: '', ami: '' })

  let form = $state<Profile>(blank())
  let secrets = $state<Secrets>(noSecrets())
  let results = $state<ChannelResult[]>([])
  let note = $state('')
  let confirmDelete = $state(false)
  const has = $derived(app.profiles.find((v) => v.profile.id === form.id)?.has)

  function edit(id: string) {
    const v = app.profiles.find((v) => v.profile.id === id)
    if (!v) return
    form = $state.snapshot(v.profile)
    secrets = noSecrets()
    results = []
    note = ''
    confirmDelete = false
  }

  function fresh() {
    form = blank()
    secrets = noSecrets()
    results = []
    note = ''
    confirmDelete = false
  }

  // The form follows the active PBX picked in the header block.
  let shown = ''
  $effect(() => {
    const id = app.active
    if (id && id !== shown) {
      shown = id
      untrack(() => edit(id))
    }
  })

  async function save(): Promise<boolean> {
    try {
      const p = await ProfileService.Save($state.snapshot(form), $state.snapshot(secrets))
      await reload()
      edit(p.id)
      note = 'Perfil guardado.'
      return true
    } catch (e) {
      note = errText(e)
      await reload()
      return false
    }
  }

  async function runTest() {
    if (!form.id) return
    app.busy = true
    results = []
    try {
      app.folio = await ProfileService.NextFolio()
      results = (await ProfileService.Test(form.id)) ?? []
      await tick()
      // Let the technician watch the stamps land without scrolling by hand.
      document.querySelector('.outcome')?.scrollIntoView({ block: 'nearest' })
    } catch (e) {
      note = errText(e)
    } finally {
      app.busy = false
    }
  }

  async function saveAndTest() {
    if (await save()) await runTest()
  }

  async function trust(r: ChannelResult) {
    try {
      await ProfileService.TrustFingerprint(form.id, r.channel, r.fingerprint)
      await reload()
      edit(form.id)
      await runTest()
    } catch (e) {
      note = errText(e)
    }
  }

  async function remove() {
    if (!confirmDelete) {
      confirmDelete = true
      setTimeout(() => (confirmDelete = false), 4000)
      return
    }
    try {
      await ProfileService.Delete(form.id)
      await reload()
      fresh()
      note = 'Perfil eliminado.'
    } catch (e) {
      note = errText(e)
    }
  }

  const resultFor = (ch: string) => results.find((r) => r.channel === ch)
</script>

{#snippet outcome(ch: string, title: string, enabled: boolean, tilt: string)}
  {@const r = resultFor(ch)}
  <div class="outcome" class:fail={r && !r.ok && !r.skipped} aria-live="polite">
    <span class="lbl">{title}</span>
    {#if !enabled}
      <span class="pending">Deshabilitado</span>
    {:else if app.busy && !r}
      <span class="pending">Probando…</span>
    {:else if !r}
      <span class="pending">Sin probar</span>
    {:else if !r.skipped}
      <span class="stamp-row">
        <span class="stamp" class:bad={!r.ok} style:--r={tilt}>{r.ok ? 'Conectado' : 'Falló'}</span>
        <span class="ms">{r.millis} ms</span>
      </span>
      <span class="msg">{r.message}</span>
      {#if r.fingerprint}
        <code class="fp">{r.fingerprint}</code>
        <button type="button" class="btn small" onclick={() => trust(r)}>
          {r.fingerprintChanged ? 'Confiar en la nueva huella' : 'Confiar en esta huella'}
        </button>
      {/if}
    {/if}
  </div>
{/snippet}

<div class="page">
  <div class="title"><span class="code">F-01</span><h1>Conexiones</h1></div>
  <p class="instr">
    Registre cada PBX y verifique sus tres canales. Las contraseñas quedan en el Administrador de
    credenciales de Windows, nunca en archivos.
  </p>

  {#if app.recovered}
    <p class="notice">
      El archivo de perfiles estaba dañado. Se guardó una copia en <code>{app.recovered}</code> y se empezó en blanco.
    </p>
  {/if}

  <h2 class="lbl section">Perfiles registrados</h2>
  <table class="ledger">
    <thead>
      <tr>
        <th class="cell-state"><span class="sr">Activa</span></th>
        <th class="lbl">Nº</th>
        <th class="lbl">Nombre</th>
        <th class="lbl">Host</th>
        <th><span class="sr">Acciones</span></th>
      </tr>
    </thead>
    <tbody>
      {#each app.profiles as v, i (v.profile.id)}
        <tr class:editing={v.profile.id === form.id}>
          <td class="cell-state">
            <span class="mark" class:filled={v.profile.id === app.active} title={v.profile.id === app.active ? 'PBX activa' : ''}></span>
          </td>
          <td class="num">{String(i + 1).padStart(2, '0')}</td>
          <td><button class="link" onclick={() => edit(v.profile.id)}>{v.profile.name}</button></td>
          <td>{v.profile.host}</td>
          <td class="row-act">
            {#if v.profile.id !== app.active}
              <button class="link" onclick={() => setActive(v.profile.id)}>Usar</button>
            {/if}
          </td>
        </tr>
      {:else}
        <tr><td colspan="5" class="empty">Sin perfiles todavía. Llene el formulario de abajo para registrar la primera PBX.</td></tr>
      {/each}
    </tbody>
  </table>

  <h2 class="lbl section">{form.id ? 'Datos del perfil' : 'Nuevo perfil'}</h2>
  <form id="perfil" class="sheetform" onsubmit={(e) => { e.preventDefault(); saveAndTest() }}>
    <label class="field half">
      <span class="lbl">Nombre</span>
      <input bind:value={form.name} required placeholder="Hospital General" />
    </label>
    <label class="field half">
      <span class="lbl">Host (nombre o IP)</span>
      <input bind:value={form.host} required placeholder="10.20.1.5" spellcheck="false" />
    </label>

    <fieldset class="channel" class:off={!form.api.enabled}>
      <legend class="chan-head">
        <span class="lbl">Canal API (portal)</span>
        <label class="check"><input type="checkbox" bind:checked={form.api.enabled} /> Habilitado</label>
      </legend>
      <label class="field">
        <span class="lbl">URL base</span>
        <input bind:value={form.api.baseURL} placeholder={`http://${form.host || 'host'}`} disabled={!form.api.enabled} spellcheck="false" />
      </label>
      <label class="field">
        <span class="lbl">Usuario</span>
        <input bind:value={form.api.user} disabled={!form.api.enabled} />
      </label>
      <label class="field">
        <span class="lbl">Contraseña</span>
        <input type="password" bind:value={secrets.api} placeholder={has?.api ? 'guardada' : ''} disabled={!form.api.enabled} autocomplete="off" />
      </label>
      <p class="hint">El portal de CompletePBX usa HTTP sin cifrar: la contraseña viaja en claro por la red.</p>
    </fieldset>

    <fieldset class="channel" class:off={!form.ssh.enabled}>
      <legend class="chan-head">
        <span class="lbl">Canal SSH</span>
        <label class="check"><input type="checkbox" bind:checked={form.ssh.enabled} /> Habilitado</label>
      </legend>
      <div class="pair">
        <label class="field">
          <span class="lbl">Puerto</span>
          <input type="number" min="1" max="65535" bind:value={form.ssh.port} disabled={!form.ssh.enabled} />
        </label>
        <label class="field">
          <span class="lbl">Usuario</span>
          <input bind:value={form.ssh.user} disabled={!form.ssh.enabled} />
        </label>
      </div>
      <label class="field">
        <span class="lbl">Contraseña</span>
        <input type="password" bind:value={secrets.ssh} placeholder={has?.ssh ? 'guardada' : ''} disabled={!form.ssh.enabled} autocomplete="off" />
      </label>
      <label class="field">
        <span class="lbl">Llave privada (ruta, opcional)</span>
        <input bind:value={form.ssh.keyPath} placeholder="C:\Users\…\.ssh\id_ed25519" disabled={!form.ssh.enabled} spellcheck="false" />
      </label>
      <label class="field">
        <span class="lbl">Frase de la llave</span>
        <input type="password" bind:value={secrets.sshKey} placeholder={has?.sshKey ? 'guardada' : ''} disabled={!form.ssh.enabled} autocomplete="off" />
      </label>
    </fieldset>

    <fieldset class="channel" class:off={!form.ami.enabled}>
      <legend class="chan-head">
        <span class="lbl">Canal AMI</span>
        <label class="check"><input type="checkbox" bind:checked={form.ami.enabled} /> Habilitado</label>
      </legend>
      <div class="pair">
        <label class="field">
          <span class="lbl">Puerto</span>
          <input type="number" min="1" max="65535" bind:value={form.ami.port} disabled={!form.ami.enabled} />
        </label>
        <label class="field">
          <span class="lbl">Usuario</span>
          <input bind:value={form.ami.user} disabled={!form.ami.enabled} />
        </label>
      </div>
      <label class="field">
        <span class="lbl">Secreto</span>
        <input type="password" bind:value={secrets.ami} placeholder={has?.ami ? 'guardado' : ''} disabled={!form.ami.enabled} autocomplete="off" />
      </label>
      <p class="hint">AMI viaja sin cifrar: en la PBX limite el acceso (permit/deny) a la red de los técnicos.</p>
    </fieldset>

    <!-- Three separate impressions, never one stamp pasted thrice. -->
    {@render outcome('api', 'Prueba API', form.api.enabled, '-3deg')}
    {@render outcome('ssh', 'Prueba SSH', form.ssh.enabled, '-1.5deg')}
    {@render outcome('ami', 'Prueba AMI', form.ami.enabled, '-4.5deg')}
  </form>

  <div class="actions">
    <button class="btn primary" type="submit" form="perfil" disabled={app.busy}>Guardar y probar</button>
    <button class="btn" type="button" onclick={save} disabled={app.busy}>Guardar</button>
    {#if form.id}
      <button class="btn" type="button" onclick={fresh} disabled={app.busy}>Nuevo perfil</button>
      <button class="btn danger" type="button" onclick={remove} disabled={app.busy}>
        {confirmDelete ? 'Confirmar: eliminar' : 'Eliminar'}
      </button>
    {/if}
    <span class="note" role="status">{note}</span>
  </div>
</div>

<style>
  .page { padding: 20px 32px 0; }
  .title { display: flex; align-items: center; gap: 14px; }
  .title .code {
    padding: 5px 8px 4px; border: 2px solid var(--ink);
    font: 700 15px/1 var(--f-label); letter-spacing: 0.06em; color: var(--ink);
  }
  h1 { font: 700 30px/1 var(--f-label); letter-spacing: 0.03em; text-transform: uppercase; color: var(--ink); }
  .instr { max-width: 72ch; margin: 10px 0 16px; color: var(--ink-2); font-size: 14px; }
  .notice { margin-bottom: 24px; padding: 10px 14px; border: 1px solid var(--fail); background: var(--pink); }
  .notice code { font: 12px var(--f-mono); }
  .section { display: block; margin-bottom: 8px; }
  .sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }

  .ledger { width: 100%; margin-bottom: 20px; border-collapse: collapse; border-block: 2px solid var(--ink); }
  .ledger th { padding: 8px 10px 6px; border-bottom: 1px solid var(--ink); text-align: left; }
  .ledger td { padding: 9px 10px; border-bottom: 1px solid var(--hair); }
  .ledger tbody tr:last-child td { border-bottom: 0; }
  .ledger tr.editing td { background: var(--tint); }
  .cell-state { width: 34px; }
  .mark { display: block; width: 12px; height: 12px; border: 2px solid var(--ink); }
  .mark.filled { background: var(--ink); }
  .num { width: 48px; color: var(--ink-2); font-weight: 500; }
  .row-act { width: 80px; text-align: right; }
  .empty { padding: 18px 10px; color: var(--ink-2); }

  .sheetform {
    display: grid; grid-template-columns: repeat(6, minmax(0, 1fr));
    border-top: 2px solid var(--ink); border-left: 1px solid var(--ink);
  }
  .field {
    display: grid; gap: 6px; min-width: 0; padding: 8px 12px 10px;
    border-right: 1px solid var(--ink); border-bottom: 1px solid var(--ink);
  }
  .field.half { grid-column: span 3; }
  .field input {
    width: 100%; min-width: 0; padding: 0; border: 0; background: transparent;
    font: 500 16px/1.3 var(--f-data); color: var(--data); outline-offset: 4px;
  }
  .field input::placeholder { color: var(--ink-2); font-style: italic; font-weight: 400; }
  .field input:disabled { color: var(--ink-2); }

  .channel {
    grid-column: span 2; display: grid; align-content: start; min-width: 0;
    margin: 0; padding: 0; border: 0;
    border-right: 1px solid var(--ink); border-bottom: 1px solid var(--ink);
  }
  .channel .field { border-right: 0; }
  /* The fieldset draws the column's bottom rule; a last field's own rule would double it. */
  .channel > :last-child { border-bottom: 0; }
  .channel.off .field { background: var(--tint); }
  .chan-head {
    float: left; width: 100%;
    display: flex; align-items: center; justify-content: space-between;
    padding: 8px 12px; border-bottom: 1px solid var(--ink); background: var(--tint);
  }
  .chan-head + * { clear: both; }
  .check { display: inline-flex; align-items: center; gap: 6px; color: var(--ink); font-size: 13px; }
  .check input { width: 16px; height: 16px; margin: 0; accent-color: var(--ink); }
  .pair { display: grid; grid-template-columns: 96px minmax(0, 1fr); }
  .pair .field:first-child { border-right: 1px solid var(--ink); }
  .hint { padding: 8px 12px; color: var(--ink-2); font-size: 12px; line-height: 1.35; }

  .outcome {
    grid-column: span 2; display: grid; align-content: start; justify-items: start; gap: 10px;
    min-height: 96px; padding: 10px 12px 14px;
    border-right: 1px solid var(--ink); border-bottom: 2px solid var(--ink);
  }
  .outcome.fail { background: var(--pink); }
  .pending { color: var(--ink-2); font: 600 12px/1 var(--f-label); letter-spacing: 0.09em; text-transform: uppercase; }
  .stamp-row { display: flex; align-items: center; gap: 12px; }
  .msg { font-size: 13px; line-height: 1.35; }
  .ms { color: var(--ink-2); font: 600 11px/1 var(--f-label); letter-spacing: 0.09em; white-space: nowrap; }
  .outcome.fail .ms { color: var(--data); }
  .fp { max-width: 100%; font: 400 11px/1.45 var(--f-mono); word-break: break-all; }

  /* The signature line of the form: always in reach, pinned to the bottom of the sheet. */
  .actions {
    position: sticky; bottom: 0; z-index: 1;
    display: flex; flex-wrap: wrap; align-items: center; gap: 10px;
    margin-top: 20px; padding: 12px 0 16px;
    border-top: 2px solid var(--ink); background: var(--paper);
  }
  .note { color: var(--ink-2); font-size: 14px; }
</style>
