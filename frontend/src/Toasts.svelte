<script lang="ts">
  import Icon from './Icon.svelte'
  import { app, dismiss } from './state.svelte'

  const icon = { ok: 'check', fail: 'alert', info: 'refresh', update: 'download' } as const
</script>

<!-- Notices pinned under the PBX sign, top right: every action says how it ended. -->
<div class="toasts" aria-live="polite">
  {#each app.toasts as t (t.id)}
    <div class="toast {t.kind}" role={t.kind === 'fail' ? 'alert' : 'status'}>
      <span class="ico"><Icon name={icon[t.kind]} /></span>
      <p class="text">{t.text}</p>
      {#if t.action}
        {@const action = t.action}
        <button class="btn lit small" onclick={() => { dismiss(t.id); action.run() }}>{action.label}</button>
      {/if}
      <button class="close" onclick={() => dismiss(t.id)} aria-label="Cerrar aviso"><Icon name="x" size={18} /></button>
    </div>
  {/each}
</div>

<style>
  .toasts {
    position: fixed; top: calc(var(--head-h, 0px) + 12px); right: 20px; z-index: 10;
    display: grid; gap: 10px; width: min(400px, calc(100vw - 260px));
  }
  .toast {
    display: flex; align-items: center; gap: 10px;
    padding: 10px 10px 10px 12px; border: 2px solid var(--rule-strong); border-radius: 10px;
    background: var(--panel); color: var(--ink);
    box-shadow: 0 6px 18px rgb(35 31 32 / 0.16);
    animation: toast-in 220ms var(--ease-out) both;
  }
  .toast.ok { border-color: var(--ok); }
  .toast.ok .ico { color: var(--ok); }
  .toast.fail { border-color: var(--fail); background: var(--fail-soft); }
  .toast.fail .ico { color: var(--fail); }
  .toast.info .ico { color: var(--sign); }
  /* A new version: the one notice in sign blue, with its action. */
  .toast.update { flex-wrap: wrap; border-color: var(--sign); background: var(--sign); color: var(--on-sign); }
  .toast.update .close { color: var(--on-sign); }
  .toast.update .close:focus-visible, .toast.update .btn:focus-visible { outline-color: var(--lit); }
  .ico { display: inline-flex; flex: none; }
  .text { flex: 1; min-width: 0; font-size: 14px; line-height: 1.35; font-weight: 600; overflow-wrap: anywhere; }
  .close { display: inline-grid; place-items: center; flex: none; width: 32px; height: 32px; padding: 0; border: 0; border-radius: 8px; background: none; color: var(--mute); cursor: pointer; }
  .close:hover { background: rgb(35 31 32 / 0.08); }
  @keyframes toast-in { from { opacity: 0; transform: translateX(24px); } }
</style>
