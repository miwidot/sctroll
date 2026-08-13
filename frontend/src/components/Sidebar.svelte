<script>
  import { i18n } from '../i18n'
  import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'

  export let currentTab = 'actions'

  $: tabs = [
    { id: 'actions', label: $i18n('nav.actions'), icon: 'A' },
    { id: 'twitch', label: $i18n('nav.twitch'), icon: 'T' },
    { id: 'log', label: $i18n('nav.log'), icon: 'L' },
    { id: 'settings', label: $i18n('nav.settings'), icon: 'S' },
  ]

  // Externe Links gehen in den Systembrowser, nicht ins App-Fenster --
  // sonst haengt der Nutzer in einem WebView ohne Zurueck-Knopf fest.
  const links = [
    { label: 'CitizenHQ',   url: 'https://citizenhq.space' },
    { label: 'Handelsnetz', url: 'https://citizenhq.space/trade' },
    { label: 'Tools',       url: 'https://citizenhq.space/tools/scu' },
    { label: 'Discord',     url: 'https://discord.gg/mhG85DuhMa' },
  ]
</script>

<nav class="sidebar">
  <div class="nav-items">
    <span class="section-label">{$i18n('nav.section.app')}</span>
    {#each tabs as tab}
      <button
        class="nav-item"
        class:active={currentTab === tab.id}
        on:click={() => currentTab = tab.id}
      >
        <span class="nav-icon chamfer-sm">{tab.icon}</span>
        <span class="nav-label">{tab.label}</span>
      </button>
    {/each}

    <span class="section-label links-label">{$i18n('nav.section.links')}</span>
    {#each links as link}
      <button class="nav-item link-item" on:click={() => BrowserOpenURL(link.url)}>
        <span class="nav-label">{link.label}</span>
        <span class="ext">↗</span>
      </button>
    {/each}
  </div>

  <div class="sidebar-footer">
    <span class="version">SCTROLL v1.0.7</span>
  </div>
</nav>

<style>
  .sidebar {
    width: 190px;
    background: var(--bg-secondary);
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    justify-content: space-between;
  }

  .nav-items {
    display: flex;
    flex-direction: column;
    padding: 14px 10px;
    gap: 2px;
  }

  .section-label {
    font-family: var(--font-display);
    text-transform: uppercase;
    letter-spacing: 0.22em;
    font-size: 9px;
    color: var(--data-dim);
    padding: 0 14px 8px;
  }
  .links-label { padding-top: 22px; }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 9px 14px;
    background: none;
    border: none;
    border-left: 2px solid transparent;
    color: var(--text-secondary);
    font-family: var(--font-display);
    font-size: 12px;
    font-weight: 500;
    letter-spacing: 0.10em;
    text-transform: uppercase;
    cursor: pointer;
    transition: all 0.18s var(--ease-out-quint);
    text-align: left;
  }

  .nav-item:hover {
    color: var(--text-primary);
    background: rgba(158, 191, 200, 0.05);
  }

  .nav-item.active {
    color: var(--accent);
    background: linear-gradient(90deg, rgba(124, 197, 209, 0.14), transparent);
    border-left-color: var(--accent);
  }

  .nav-icon {
    width: 26px;
    height: 26px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 10px;
    font-weight: 700;
    background: var(--bg-card);
    border: 1px solid var(--border);
    color: inherit;
    flex-shrink: 0;
  }

  .nav-item.active .nav-icon {
    background: rgba(124, 197, 209, 0.14);
    border-color: rgba(124, 197, 209, 0.45);
    color: var(--accent);
  }

  .nav-label { flex: 1; }

  .link-item {
    font-size: 11px;
    letter-spacing: 0.14em;
    color: var(--text-muted);
    padding-left: 16px;
  }
  .link-item:hover { color: var(--data-bright); }

  .ext {
    font-size: 10px;
    opacity: 0.6;
  }

  .sidebar-footer {
    padding: 12px 16px;
    border-top: 1px solid var(--border);
  }
  .version {
    font-family: var(--font-mono);
    font-size: 10px;
    letter-spacing: 0.10em;
    color: var(--data-dim);
  }
</style>
