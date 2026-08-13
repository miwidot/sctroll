<script>
  import { onMount, onDestroy } from 'svelte'
  import { CheckForUpdate, InstallUpdate } from '../../wailsjs/go/main/App'
  import { EventsOn, BrowserOpenURL } from '../../wailsjs/runtime/runtime'

  let release = null
  let dismissed = false
  let installing = false
  let progress = 0
  let error = ''
  let unsubscribers = []

  onMount(async () => {
    unsubscribers.push(EventsOn('update-available', (r) => {
      release = r
      dismissed = false
    }))
    unsubscribers.push(EventsOn('update-progress', (p) => {
      progress = p
    }))
  })

  onDestroy(() => unsubscribers.forEach(fn => fn()))

  async function install() {
    installing = true
    error = ''
    progress = 0
    try {
      // Läuft durch: lädt, prüft Signatur, tauscht aus und startet neu.
      await InstallUpdate()
    } catch (e) {
      error = String(e)
      installing = false
    }
  }

  export async function check() {
    try {
      const r = await CheckForUpdate()
      release = r
      dismissed = !r.newer
      return r
    } catch (e) {
      error = String(e)
      return null
    }
  }
</script>

{#if release && release.newer && !dismissed}
  <div class="update-bar">
    <div class="left">
      <span class="tag">Update</span>
      <span class="text">
        <strong>Version {release.version}</strong> ist verfügbar — du hast {release.published_at ? release.published_at : ''}
      </span>
      <button class="linklike" on:click={() => BrowserOpenURL(release.url)}>Was ist neu?</button>
    </div>

    <div class="right">
      {#if installing}
        <div class="progress">
          <div class="bar" style="width: {Math.round(progress * 100)}%"></div>
        </div>
        <span class="pct">{Math.round(progress * 100)} %</span>
      {:else}
        <button class="btn accent" on:click={install}>Jetzt aktualisieren</button>
        <button class="btn ghost" on:click={() => dismissed = true}>Später</button>
      {/if}
    </div>
  </div>

  {#if installing}
    <div class="note">Lädt, prüft die Signatur und startet danach neu.</div>
  {/if}
  {#if error}
    <div class="note error">{error}</div>
  {/if}
{/if}

<style>
  .update-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 8px 20px;
    background: linear-gradient(90deg, rgba(124, 197, 209, 0.16), rgba(124, 197, 209, 0.04));
    border-bottom: 1px solid rgba(124, 197, 209, 0.35);
  }

  .left { display: flex; align-items: center; gap: 10px; min-width: 0; }

  .tag {
    font-family: var(--font-display);
    font-size: 9px;
    letter-spacing: 0.20em;
    text-transform: uppercase;
    color: var(--space-900);
    background: var(--accent);
    padding: 2px 8px;
    flex-shrink: 0;
  }

  .text {
    font-size: 12px;
    color: var(--text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .text strong { color: var(--text-primary); }

  .right { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }

  .btn {
    padding: 5px 14px;
    background: transparent;
    border: 1px solid var(--border);
    color: var(--text-secondary);
    cursor: pointer;
    border-radius: var(--radius-sm);
    font-family: var(--font-display);
    font-size: 11px;
    letter-spacing: 0.08em;
  }
  .btn.accent {
    background: var(--accent);
    border-color: var(--accent);
    color: var(--space-900);
    font-weight: 700;
  }
  .btn.accent:hover { filter: brightness(1.1); }
  .btn.ghost:hover { border-color: var(--border-hover); color: var(--text-primary); }

  .linklike {
    background: none;
    border: none;
    padding: 0;
    color: var(--accent);
    cursor: pointer;
    font-size: 11px;
    text-decoration: underline;
    white-space: nowrap;
  }

  .progress {
    width: 160px;
    height: 6px;
    background: var(--bg-input);
    border: 1px solid var(--border);
  }
  .bar {
    height: 100%;
    background: var(--accent);
    transition: width 0.2s;
  }
  .pct {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--accent);
    min-width: 44px;
    text-align: right;
  }

  .note {
    padding: 6px 20px;
    font-size: 11px;
    color: var(--text-muted);
    background: rgba(124, 197, 209, 0.05);
    border-bottom: 1px solid var(--border);
  }
  .note.error { color: var(--danger); background: rgba(239, 84, 84, 0.07); }
</style>
