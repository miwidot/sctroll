<script>
  import { onMount, onDestroy } from 'svelte'
  import { GetGlobalEnable, SetGlobalEnable, IsTwitchConnected, GetTwitchChannel } from '../../wailsjs/go/main/App'
  import { EventsOn } from '../../wailsjs/runtime/runtime'
  import { i18n, locale } from '../i18n'

  let globalEnabled = true
  let twitchConnected = false
  let channelName = ''
  let unsubscribers = []

  onMount(async () => {
    globalEnabled = await GetGlobalEnable()
    twitchConnected = await IsTwitchConnected()
    if (twitchConnected) {
      channelName = await GetTwitchChannel()
    }

    unsubscribers.push(EventsOn('twitch-connected', () => {
      twitchConnected = true
      GetTwitchChannel().then(n => channelName = n)
    }))
    unsubscribers.push(EventsOn('twitch-disconnected', () => {
      twitchConnected = false
    }))
    unsubscribers.push(EventsOn('twitch-authenticated', (name) => {
      channelName = name
    }))
    unsubscribers.push(EventsOn('global-toggle', (val) => {
      globalEnabled = val
    }))
  })

  onDestroy(() => {
    unsubscribers.forEach(fn => fn())
  })

  async function toggleGlobal() {
    globalEnabled = !globalEnabled
    await SetGlobalEnable(globalEnabled)
  }
</script>

<div class="top-bar">
  <div class="top-left">
    <span class="sys-label">SYSTEM STATUS:</span>
    <span class="sys-status" class:online={globalEnabled}>
      {globalEnabled ? 'OPERATIONAL' : 'PAUSED'}
    </span>
  </div>
  <div class="top-right">
    <span class="twitch-status" class:online={twitchConnected}>
      {twitchConnected ? channelName : $i18n('app.twitch_offline')}
    </span>
  </div>
</div>

<header class="header">
  <div class="header-left">
    <div class="logo">
      <span class="logo-icon chamfer-sm">
        <span class="logo-icon-t1">S</span><span class="logo-icon-t2">C</span>
      </span>
      <div class="logo-text">
        <span class="logo-sc">{$i18n('app.title.1')}</span>
        <span class="logo-troll">{$i18n('app.title.2')}</span>
      </div>
    </div>
  </div>

  <div class="header-right">
    <button class="master-toggle chamfer-sm" class:enabled={globalEnabled} on:click={toggleGlobal}>
      <span class="toggle-dot"></span>
      {globalEnabled ? $i18n('app.active') : $i18n('app.off')}
    </button>
  </div>
</header>

<style>
  .top-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 4px 20px;
    background: var(--bg-primary);
    border-bottom: 1px solid var(--border);
    height: 28px;
    font-size: 10px;
    font-weight: 600;
    letter-spacing: 1px;
    text-transform: uppercase;
    -webkit-app-region: drag;
  }
  .top-left, .top-right {
    display: flex;
    align-items: center;
    gap: 8px;
    -webkit-app-region: no-drag;
  }
  .sys-label { color: var(--text-muted); }
  .sys-status {
    color: var(--danger);
    display: flex;
    align-items: center;
    gap: 5px;
  }
  .sys-status::before {
    content: '';
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--danger);
  }
  .sys-status.online { color: var(--accent); }
  .sys-status.online::before {
    background: var(--accent);
    box-shadow: 0 0 8px rgba(124, 197, 209, 0.5);
  }

  .twitch-status {
    color: var(--text-muted);
    display: flex;
    align-items: center;
    gap: 5px;
  }
  .twitch-status::before {
    content: '';
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--text-muted);
  }
  .twitch-status.online { color: #9147ff; }
  .twitch-status.online::before {
    background: #9147ff;
    box-shadow: 0 0 6px rgba(145, 71, 255, 0.5);
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 20px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
    height: 52px;
  }

  .header-left, .header-right {
    display: flex;
    align-items: center;
  }

  .logo {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .logo-icon {
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, rgba(124, 197, 209, 0.15), rgba(124, 197, 209, 0.20));
    border: 1px solid rgba(124, 197, 209, 0.50);
        font-size: 13px;
    font-weight: 900;
    letter-spacing: -1px;
    box-shadow: 0 0 12px rgba(124, 197, 209, 0.20), inset 0 0 8px rgba(124, 197, 209, 0.08);
  }
  .logo-icon-t1 {
    color: var(--accent);
    text-shadow: 0 0 8px rgba(124, 197, 209, 0.6);
  }
  .logo-icon-t2 {
    color: var(--brand);
    text-shadow: 0 0 8px rgba(215, 92, 87, 0.5);
  }
  .logo-text {
    display: flex;
    align-items: baseline;
    gap: 6px;
  }
  .logo-sc {
    font-size: 18px;
    font-weight: 900;
    color: var(--ink);
    letter-spacing: 4px;
    text-transform: uppercase;
  }
  .logo-troll {
    font-size: 16px;
    font-weight: 700;
    color: var(--brand);
    letter-spacing: 2px;
    text-transform: uppercase;
    text-shadow: 0 0 10px rgba(215, 92, 87, 0.3);
  }

  .master-toggle {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 16px;
    border: 1px solid var(--danger);
    background: var(--danger-dim);
    color: var(--danger);
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 2px;
    cursor: pointer;
    border-radius: var(--radius-sm);
    transition: all 0.2s;
  }
  .toggle-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--danger);
    transition: all 0.3s;
  }
  .master-toggle.enabled {
    border-color: var(--accent);
    background: rgba(124, 197, 209, 0.10);
    color: var(--accent);
  }
  .master-toggle.enabled .toggle-dot {
    background: var(--accent);
    box-shadow: 0 0 8px rgba(124, 197, 209, 0.5);
  }
  .master-toggle:hover {
    filter: brightness(1.15);
  }
</style>
