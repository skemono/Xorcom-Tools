<script lang="ts">
  // The sign pictograms: authored line icons on one 24px grid, 2px stroke, round ends.
  let { name, size = 20 }: { name: string; size?: number } = $props()
  const keys = [[6, 4], [12, 4], [18, 4], [6, 10], [12, 10], [18, 10], [6, 16], [12, 16], [18, 16], [12, 21.5]]
</script>

<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
  {#if name === 'phone'}
    <path d="M5.5 3.5h3l1.8 4.6-2.3 1.5a11 11 0 0 0 6.4 6.4l1.5-2.3 4.6 1.8v3a2 2 0 0 1-2 2A16.5 16.5 0 0 1 3.5 5.5a2 2 0 0 1 2-2z" />
  {:else if name === 'plug'}
    <path d="M9 2.5v4.5M15 2.5v4.5M6.5 7h11v3.5a5.5 5.5 0 0 1-11 0zM12 16v5.5" />
  {:else if name === 'keypad'}
    {#each keys as [x, y]}<circle cx={x} cy={y} r="1.6" fill="currentColor" stroke="none" />{/each}
  {:else if name === 'check'}
    <path d="M5 12.5l4.5 4.5L19 7.5" />
  {:else if name === 'chevron-right'}
    <path d="M9 5.5l6.5 6.5L9 18.5" />
  {:else if name === 'chevron-down'}
    <path d="M5.5 9l6.5 6.5L18.5 9" />
  {:else if name === 'arrow-right'}
    <path d="M4.5 12h15M13.5 6l6 6-6 6" />
  {:else if name === 'x'}
    <path d="M6.5 6.5l11 11M17.5 6.5l-11 11" />
  {:else if name === 'alert'}
    <path d="M12 3.5L21.5 20h-19zM12 10v4.5M12 17.4v.1" />
  {:else if name === 'refresh'}
    <path d="M19.5 12a7.5 7.5 0 1 1-2.2-5.3M19.5 4v5h-5" />
  {:else if name === 'folder'}
    <path d="M3 6.5a1.5 1.5 0 0 1 1.5-1.5h4.6l2 2h8.4a1.5 1.5 0 0 1 1.5 1.5v9a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 17.5z" />
  {:else if name === 'download'}
    <path d="M12 3.5v11M7 10l5 5 5-5M4.5 20h15" />
  {:else if name === 'reload'}
    <path d="M4.5 12a7.5 7.5 0 0 1 12.8-5.3L19.5 9M19.5 4v5h-5M19.5 12a7.5 7.5 0 0 1-12.8 5.3L4.5 15M4.5 20v-5h5" />
  {/if}
</svg>
