<script>
  import { onMount, onDestroy } from 'svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime'
  import { i18n } from '../i18n'

  let events = []
  let unsubscribers = []

  onMount(() => {
    unsubscribers.push(EventsOn('action-executed', (data) => {
      addEvent('action', `${data.user} ${$i18n('log.action_triggered')} "${data.action}"`)
    }))
    unsubscribers.push(EventsOn('action-error', (data) => {
      addEvent('error', `${$i18n('log.action_error')} ${data.action}: ${data.error}`)
    }))
    unsubscribers.push(EventsOn('key-blocked', (key) => {
      addEvent('lock', `${$i18n('log.key_blocked')}: ${key.toUpperCase()}`)
    }))
    unsubscribers.push(EventsOn('twitch-log', (msg) => {
      addEvent('twitch', msg)
    }))
    unsubscribers.push(EventsOn('twitch-connected', () => {
      addEvent('twitch', $i18n('log.twitch_connected'))
    }))
    unsubscribers.push(EventsOn('twitch-disconnected', (msg) => {
      addEvent('twitch', $i18n('log.twitch_disconnected') + ': ' + (msg || ''))
    }))
  })

  onDestroy(() => unsubscribers.forEach(fn => fn()))

  function addEvent(type, message) {
    events = [...events, {
      time: new Date().toLocaleTimeString(),
      type,
      message
    }].slice(-200)
  }

  function clearLog() {
    events = []
  }
</script>

<div class="panel">
  <div class="panel-header">
    <h2>{$i18n('log.title')}</h2>
    <button class="clear-btn" on:click={clearLog}>{$i18n('log.clear')}</button>
  </div>

  <div class="log-container">
    {#each [...events].reverse() as event}
      <div class="log-entry {event.type}">
        <span class="log-time">{event.time}</span>
        <span class="log-type">{event.type.toUpperCase()}</span>
        <span class="log-msg">{event.message}</span>
      </div>
    {:else}
      <div class="empty">{$i18n('log.empty')}</div>
    {/each}
  </div>
</div>

<style>
  .panel { height: 100%; display: flex; flex-direction: column; }
  .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 14px;
  }
  .panel-header h2 {
    font-size: 20px;
    font-weight: 700;
    color: var(--accent);
    letter-spacing: 1px;
  }
  .clear-btn {
    padding: 6px 14px;
    background: transparent;
    border: 1px solid var(--border);
    color: var(--text-secondary);
    font-size: 11px;
    cursor: pointer;
    border-radius: var(--radius-sm);
    transition: all 0.15s;
  }
  .clear-btn:hover { border-color: var(--accent); color: var(--accent); }

  .log-container {
    flex: 1;
    overflow-y: auto;
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 12px;
    font-family: 'Consolas', 'Courier New', monospace;
    font-size: 12px;
  }

  .log-entry {
    padding: 5px 8px;
    margin-bottom: 2px;
    border-radius: 4px;
    display: flex;
    gap: 10px;
    align-items: baseline;
    transition: background 0.1s;
  }
  .log-entry:hover { background: rgba(255,255,255,0.02); }
  .log-time { color: var(--text-muted); min-width: 72px; font-size: 11px; }
  .log-type {
    min-width: 60px;
    font-weight: 700;
    font-size: 10px;
    padding: 1px 6px;
    border-radius: 3px;
    text-align: center;
  }
  .log-msg { color: var(--text-secondary); }

  .log-entry.action .log-type { color: var(--success); background: var(--success-dim); }
  .log-entry.error .log-type { color: var(--danger); background: var(--danger-dim); }
  .log-entry.lock .log-type { color: var(--warning); background: rgba(245, 158, 11, 0.12); }
  .log-entry.twitch .log-type { color: #9147ff; background: rgba(145, 71, 255, 0.12); }

  .empty {
    color: var(--text-muted);
    text-align: center;
    padding: 60px 20px;
    font-family: 'Inter', 'Segoe UI', sans-serif;
    font-size: 14px;
  }
</style>
