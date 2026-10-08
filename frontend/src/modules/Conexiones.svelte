<script lang="ts">
  import { tick, untrack } from 'svelte'
  import { ProfileService } from '../../bindings/github.com/skemono/Xorcom-Tools'
  import type { ChannelResult, Profile, Secrets } from '../../bindings/github.com/skemono/Xorcom-Tools/pbx'
  import Icon from '../Icon.svelte'
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
    const id = form.id
    app.busy = true
    results = []
    try {
      const res = (await ProfileService.Test(id)) ?? []
      if (form.id !== id) return // the sheet changed under the test: never stamp A's results on B
      results = res
      await tick()
      // Let the technician watch the stamps land without scrolling by hand.
      document.querySelector('.outcomes')?.scrollIntoView({ block: 'center' })
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
    <span class="out-name">{title}</span>
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
  <h1>Conexiones</h1>
  <p class="lead">Registre cada PBX y pruebe sus tres canales. Las contraseñas quedan cifradas en Windows.</p>

  {#if app.recovered}
    <p class="notice"><Icon name="alert" />
      <span>El archivo de perfiles estaba dañado. Se guardó una copia en <code>{app.recovered}</code> y se empezó en blanco.</span>
    </p>
  {/if}

  <section class="registry">
    <h2 class="reg-head">PBX registradas</h2>
    <ul class="ledger">
      {#each app.profiles as v (v.profile.id)}
        <li class:editing={v.profile.id === form.id}>
          <button class="name" onclick={() => edit(v.profile.id)} disabled={app.busy}>{v.profile.name}</button>
          <code class="host">{v.profile.host}</code>
          {#if v.profile.id === app.active}
            <span class="active-pill"><Icon name="check" size={16} />Activa</span>
          {:else}
            <button class="btn small" onclick={() => setActive(v.profile.id)} disabled={app.busy}>Usar</button>
          {/if}
        </li>
      {:else}
        <li class="empty">Sin perfiles todavía. Complete los pasos de abajo para registrar la primera PBX.</li>
      {/each}
    </ul>
  </section>

  <form id="perfil" class="steps" onsubmit={(e) => { e.preventDefault(); saveAndTest() }}>
    <section class="step">
      <h2 class="step-head"><span class="disc">1</span>{form.id ? 'Datos de la PBX' : 'Nueva PBX'}</h2>
      <div class="pair-wide">
        <label class="field">
          <span class="field-name">Nombre</span>
          <input class="control" bind:value={form.name} required placeholder="Hospital General" />
        </label>
        <label class="field">
          <span class="field-name">Host (nombre o IP)</span>
          <input class="control" bind:value={form.host} required placeholder="10.20.1.5" spellcheck="false" />
        </label>
      </div>
    </section>

    <section class="step">
      <h2 class="step-head"><span class="disc">2</span>Canales de acceso<span class="aside">Habilite los que use esta PBX</span></h2>
      <div class="chan-grid">
        <fieldset class="channel" class:off={!form.api.enabled}>
          <legend class="chan-head">
            <span>API (portal)</span>
            <label class="check"><input type="checkbox" bind:checked={form.api.enabled} /> Habilitado</label>
          </legend>
          <label class="field">
            <span class="field-name">URL base</span>
            <input class="control" bind:value={form.api.baseURL} placeholder={`http://${form.host || 'host'}`} disabled={!form.api.enabled} spellcheck="false" />
          </label>
          <label class="field">
            <span class="field-name">Usuario</span>
            <input class="control" bind:value={form.api.user} disabled={!form.api.enabled} />
          </label>
          <label class="field">
            <span class="field-name">Contraseña</span>
            <input class="control" type="password" bind:value={secrets.api} placeholder={has?.api ? 'guardada' : ''} disabled={!form.api.enabled} autocomplete="off" />
          </label>
          <p class="hint">El portal usa HTTP sin cifrar: la contraseña viaja en claro.</p>
        </fieldset>

        <fieldset class="channel" class:off={!form.ssh.enabled}>
          <legend class="chan-head">
            <span>SSH</span>
            <label class="check"><input type="checkbox" bind:checked={form.ssh.enabled} /> Habilitado</label>
          </legend>
          <div class="pair">
            <label class="field">
              <span class="field-name">Puerto</span>
              <input class="control" type="number" min="1" max="65535" bind:value={form.ssh.port} disabled={!form.ssh.enabled} />
            </label>
            <label class="field">
              <span class="field-name">Usuario</span>
              <input class="control" bind:value={form.ssh.user} disabled={!form.ssh.enabled} />
            </label>
          </div>
          <label class="field">
            <span class="field-name">Contraseña</span>
            <input class="control" type="password" bind:value={secrets.ssh} placeholder={has?.ssh ? 'guardada' : ''} disabled={!form.ssh.enabled} autocomplete="off" />
          </label>
          <label class="field">
            <span class="field-name">Llave privada (ruta, opcional)</span>
            <input class="control mono" bind:value={form.ssh.keyPath} placeholder="C:\Users\…\.ssh\id_ed25519" disabled={!form.ssh.enabled} spellcheck="false" />
          </label>
          <label class="field">
            <span class="field-name">Frase de la llave</span>
            <input class="control" type="password" bind:value={secrets.sshKey} placeholder={has?.sshKey ? 'guardada' : ''} disabled={!form.ssh.enabled} autocomplete="off" />
          </label>
        </fieldset>

        <fieldset class="channel" class:off={!form.ami.enabled}>
          <legend class="chan-head">
            <span>AMI</span>
            <label class="check"><input type="checkbox" bind:checked={form.ami.enabled} /> Habilitado</label>
          </legend>
          <div class="pair">
            <label class="field">
              <span class="field-name">Puerto</span>
              <input class="control" type="number" min="1" max="65535" bind:value={form.ami.port} disabled={!form.ami.enabled} />
            </label>
            <label class="field">
              <span class="field-name">Usuario</span>
              <input class="control" bind:value={form.ami.user} disabled={!form.ami.enabled} />
            </label>
          </div>
          <label class="field">
            <span class="field-name">Secreto</span>
            <input class="control" type="password" bind:value={secrets.ami} placeholder={has?.ami ? 'guardado' : ''} disabled={!form.ami.enabled} autocomplete="off" />
          </label>
          <p class="hint">AMI viaja sin cifrar: en la PBX limite el acceso (permit/deny) a la red de los técnicos.</p>
        </fieldset>
      </div>
    </section>

    <section class="step">
      <h2 class="step-head"><span class="disc">3</span>Prueba de conexión<span class="aside">«Guardar y probar» la ejecuta</span></h2>
      <!-- Three separate impressions, never one stamp pasted thrice. -->
      <div class="outcomes">
        {@render outcome('api', 'API', form.api.enabled, '-3deg')}
        {@render outcome('ssh', 'SSH', form.ssh.enabled, '-1.5deg')}
        {@render outcome('ami', 'AMI', form.ami.enabled, '-4.5deg')}
      </div>
    </section>
  </form>

  <div class="actions">
    <button class="btn primary" type="submit" form="perfil" disabled={app.busy}><span class="disc">3</span>Guardar y probar</button>
    <button class="btn" type="button" onclick={save} disabled={app.busy}>Guardar</button>
    {#if form.id}
      <button class="btn" type="button" onclick={fresh} disabled={app.busy}>Nueva PBX</button>
      <!-- Two-step: the second click of a double-click (detail 2) must never confirm. -->
      <button class="btn danger" type="button" onclick={(e) => e.detail <= 1 && remove()} disabled={app.busy}>
        {confirmDelete ? 'Confirmar: eliminar' : 'Eliminar'}
      </button>
    {/if}
    <span class="note" role="status">{note}</span>
  </div>
</div>

<style>
  .page { padding: 22px 28px 0; }
  .notice { margin-bottom: 18px; }

  /* The registry: every PBX on file, the active one marked. */
  .registry { margin-bottom: 18px; }
  .reg-head { margin-bottom: 8px; font: 800 16px/1.2 var(--f); }
  .ledger { margin: 0; padding: 0; list-style: none; border: 2px solid var(--rule); border-radius: 12px; background: var(--panel); overflow: hidden; }
  .ledger li { display: flex; align-items: center; gap: 16px; min-height: 52px; padding: 6px 12px 6px 16px; }
  .ledger li + li { border-top: 1px solid var(--rule); }
  .ledger li.editing { background: var(--tint); }
  .name {
    flex: 1; min-width: 0; padding: 0; border: 0; background: none; cursor: pointer; text-align: left;
    color: var(--sign); font: 700 16px/1.3 var(--f);
    text-decoration: underline; text-decoration-thickness: 1.5px; text-underline-offset: 3px;
  }
  .name:disabled { cursor: default; opacity: 0.6; }
  .host { width: 180px; color: var(--mute); font-size: 13px; }
  .active-pill { display: inline-flex; align-items: center; gap: 4px; height: 36px; padding: 0 12px; border-radius: 99px; background: var(--sign); color: var(--on-sign); font-weight: 700; font-size: 14px; }
  .empty { color: var(--mute); }

  .steps { display: grid; gap: 14px; }
  .pair-wide { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }

  /* Three channel columns split by 2px rules: one panel, no boxes inside it. */
  .chan-grid, .outcomes { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .channel { display: grid; align-content: start; gap: 12px; min-width: 0; margin: 0; padding: 0 16px; border: 0; border-left: 2px solid var(--rule); }
  .channel:first-child, .outcome:first-child { padding-left: 0; border-left: 0; }
  .channel:last-child, .outcome:last-child { padding-right: 0; }
  .chan-head {
    display: flex; align-items: center; justify-content: space-between; gap: 8px;
    width: 100%; padding: 0 0 8px; border-bottom: 2px solid var(--rule);
    font: 800 15px/1.2 var(--f);
  }
  .chan-head .check { font-weight: 500; }
  .channel.off .chan-head > span { color: var(--mute); }
  .pair { display: grid; grid-template-columns: 92px minmax(0, 1fr); gap: 10px; }

  .outcome { display: grid; align-content: start; justify-items: start; gap: 10px; min-width: 0; min-height: 96px; padding: 2px 16px; border-left: 2px solid var(--rule); }
  .outcome.fail .out-name { color: var(--fail); }
  .out-name { font: 800 15px/1.2 var(--f); }
  .pending { color: var(--mute); font-size: 14px; }
  .stamp-row { display: flex; align-items: center; gap: 12px; }
  .msg { font-size: 14px; line-height: 1.4; }
  .outcome.fail .msg { color: var(--fail); }
  .ms { color: var(--mute); font-size: 13px; white-space: nowrap; }
  .fp { max-width: 100%; font-size: 11px; line-height: 1.45; word-break: break-all; }
</style>

