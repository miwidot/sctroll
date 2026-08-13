<script>
  import { onMount, onDestroy } from 'svelte'
  import {
    StartTwitchAuth, ConnectTwitch,
    DisconnectTwitch, IsTwitchConnected, GetTwitchChannel,
    SyncRewards, DeleteAllRewards,
    GetTwitchApp, SetTwitchApp
  } from '../../wailsjs/go/main/App'
  import { EventsOn, BrowserOpenURL } from '../../wailsjs/runtime/runtime'
  import { i18n } from '../i18n'

  let connected = false
  let channelName = ''
  let authStatus = ''
  let userCode = ''
  let verificationURI = ''
  let logs = []
  let unsubscribers = []

  // Eigene Twitch-App
  let showApp = false
  let clientID = ''
  let clientSecret = ''
  let hasSecret = false
  let isDefaultApp = true
  let appStatus = ''
  let secretWarning = ''

  async function loadApp() {
    try {
      const app = await GetTwitchApp()
      clientID = app.client_id || ''
      hasSecret = app.has_secret
      isDefaultApp = app.is_default
    } catch (e) {}
  }

  async function saveApp() {
    appStatus = ''
    try {
      await SetTwitchApp(clientID, clientSecret)
      clientSecret = ''
      secretWarning = ''
      await loadApp()
      appStatus = 'Gespeichert.'
    } catch (e) {
      appStatus = 'Fehler: ' + e
    }
  }

  onMount(async () => {
    connected = await IsTwitchConnected()
    if (connected) channelName = await GetTwitchChannel()
    await loadApp()

    unsubscribers.push(EventsOn('twitch-needs-secret', (msg) => {
      secretWarning = msg
      showApp = true
    }))

    unsubscribers.push(EventsOn('twitch-connected', () => {
      connected = true
      authStatus = $i18n('twitch.connected') + '!'
      GetTwitchChannel().then(n => channelName = n)
    }))
    unsubscribers.push(EventsOn('twitch-disconnected', (msg) => {
      connected = false
      authStatus = $i18n('twitch.disconnected') + ': ' + (msg || '')
    }))
    unsubscribers.push(EventsOn('twitch-authenticated', (name) => {
      channelName = name
      userCode = ''
      verificationURI = ''
      authStatus = $i18n('twitch.authenticated_as') + ' ' + name
    }))
    unsubscribers.push(EventsOn('twitch-error', (err) => {
      authStatus = $i18n('twitch.error') + ': ' + err
      userCode = ''
    }))
    unsubscribers.push(EventsOn('twitch-log', (msg) => {
      logs = [...logs.slice(-49), { time: new Date().toLocaleTimeString(), msg }]
    }))
  })

  onDestroy(() => unsubscribers.forEach(fn => fn()))

  async function startAuth() {
    authStatus = $i18n('twitch.auth.starting')
    try {
      const result = await StartTwitchAuth()
      userCode = result.user_code
      verificationURI = result.verification_uri
      authStatus = $i18n('twitch.auth.browser')
    } catch (e) {
      authStatus = $i18n('twitch.error') + ': ' + e
    }
  }

  async function connect() {
    authStatus = $i18n('twitch.connecting')
    try {
      await ConnectTwitch()
    } catch (e) {
      authStatus = $i18n('twitch.error') + ': ' + e
    }
  }

  async function disconnect() {
    await DisconnectTwitch()
  }

  async function syncRewards() {
    authStatus = $i18n('twitch.rewards.syncing')
    try {
      await SyncRewards()
      authStatus = $i18n('twitch.rewards.synced')
    } catch (e) {
      authStatus = $i18n('twitch.error') + ': ' + e
    }
  }

  async function deleteRewards() {
    if (confirm($i18n('twitch.rewards.delete_confirm'))) {
      await DeleteAllRewards()
      authStatus = $i18n('twitch.rewards.deleted')
    }
  }
</script>

<div class="panel">
  <h2>{$i18n('twitch.title')}</h2>

  {#if secretWarning}
    <div class="warn-box">
      <strong>Anmeldung hält nicht über Neustarts</strong>
      <p>{secretWarning}</p>
      <p class="warn-hint">
        Empfohlen: eine eigene App auf
        <button class="linklike" on:click={() => BrowserOpenURL('https://dev.twitch.tv/console/apps')}>dev.twitch.tv/console/apps</button>
        anlegen und dort <em>Client Type</em> auf <strong>Public</strong> stellen. Dann wird
        gar kein Secret gebraucht. Die Client-ID unten eintragen.
      </p>
    </div>
  {/if}

  <div class="section">
    <div class="section-header">
      <span class="section-number">1</span>
      <h3>{$i18n('twitch.auth')}</h3>
    </div>
    <div class="button-row">
      {#if !channelName}
        <button class="btn accent" on:click={startAuth}>
          {$i18n('twitch.auth.connect')}
        </button>
      {:else}
        <span class="channel-badge">{channelName}</span>
        {#if connected}
          <span class="connected-badge">{$i18n('twitch.connected')}</span>
          <button class="btn danger" on:click={disconnect}>{$i18n('twitch.disconnect')}</button>
        {:else}
          <button class="btn" on:click={startAuth}>{$i18n('twitch.auth.reconnect')}</button>
        {/if}
      {/if}
    </div>

    {#if userCode}
      <div class="device-code-box">
        <p class="device-code-hint">{$i18n('twitch.auth.device_hint')}</p>
        <div class="device-code">{userCode}</div>
        <p class="device-code-url">
          <a href={verificationURI} target="_blank">{verificationURI}</a>
        </p>
      </div>
    {/if}
  </div>

  <div class="section">
    <div class="section-header">
      <span class="section-number">2</span>
      <h3>{$i18n('twitch.rewards')}</h3>
    </div>
    <div class="button-row">
      <button class="btn accent" on:click={syncRewards} disabled={!connected}>{$i18n('twitch.rewards.sync')}</button>
      <button class="btn danger" on:click={deleteRewards} disabled={!connected}>{$i18n('twitch.rewards.delete')}</button>
    </div>
  </div>

  {#if authStatus}
    <div class="status-bar">{authStatus}</div>
  {/if}

  <div class="section">
    <button class="disclosure" on:click={() => showApp = !showApp}>
      {showApp ? '▾' : '▸'} Twitch-App
      <span class="app-state">{isDefaultApp ? 'mitgeliefert' : 'eigene'}{hasSecret ? ', Secret hinterlegt' : ''}</span>
    </button>

    {#if showApp}
      <p class="hint">
        Standardmäßig benutzt SCTroll eine mitgelieferte App. Wenn die Anmeldung nach ein paar
        Stunden verlorengeht, ist die App als <em>Confidential</em> registriert und verlangt beim
        Erneuern ein Secret.
      </p>
      <p class="hint">
        Der saubere Weg: eigene App anlegen mit <em>Client Type</em> = <strong>Public</strong> —
        die erneuert ohne Secret. OAuth Redirect URL <code>http://localhost</code> ist Pflichtfeld,
        wird aber nicht benutzt.
      </p>

      <label class="field">
        <span>Client-ID</span>
        <input type="text" bind:value={clientID} placeholder="leer lassen für die mitgelieferte App" spellcheck="false" />
      </label>
      <label class="field">
        <span>Client Secret <em>(nur bei Confidential-Apps)</em></span>
        <input type="password" bind:value={clientSecret}
               placeholder={hasSecret ? '•••••••• (hinterlegt, zum Ändern neu eingeben)' : 'leer lassen bei Public-Apps'}
               spellcheck="false" autocomplete="off" />
      </label>

      <div class="button-row">
        <button class="btn accent" on:click={saveApp}>Speichern</button>
      </div>
      {#if appStatus}<p class="app-status" class:error={appStatus.startsWith('Fehler')}>{appStatus}</p>{/if}
      <p class="hint muted">
        Achtung: Ein Wechsel der Client-ID setzt die Anmeldung zurück, und bereits angelegte
        Rewards lassen sich danach nicht mehr verwalten — sie müssen neu erstellt werden.
      </p>
    {/if}
  </div>

  {#if logs.length > 0}
    <div class="log-section">
      <h3>{$i18n('twitch.log')}</h3>
      <div class="log-entries">
        {#each logs as entry}
          <div class="log-entry">
            <span class="log-time">{entry.time}</span>
            <span>{entry.msg}</span>
          </div>
        {/each}
      </div>
    </div>
  {/if}
</div>

<style>
  .panel { max-width: 700px; }
  h2 {
    font-size: 20px;
    font-weight: 700;
    color: var(--accent);
    letter-spacing: 1px;
    margin-bottom: 22px;
  }
  h3 {
    font-size: 14px;
    color: var(--text-primary);
    font-weight: 600;
  }

  .section {
    margin-bottom: 16px;
    padding: 18px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }

  .section-header {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 14px;
  }
  .section-number {
    width: 26px;
    height: 26px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--accent-dim);
    border: 1px solid var(--accent);
    border-radius: 50%;
    font-size: 12px;
    font-weight: 700;
    color: var(--accent);
  }

  .button-row {
    display: flex;
    gap: 10px;
    align-items: center;
  }

  .btn {
    padding: 8px 18px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    color: var(--text-primary);
    cursor: pointer;
    border-radius: var(--radius-sm);
    font-size: 12px;
    font-weight: 600;
    transition: all 0.15s;
  }
  .btn:hover { border-color: var(--accent); }
  .btn:disabled { opacity: 0.35; cursor: default; pointer-events: none; }
  .btn.accent {
    background: var(--accent);
    border-color: var(--accent);
    color: #1a1a1a;
    font-weight: 700;
  }
  .btn.accent:hover { filter: brightness(1.1); }
  .btn.danger { border-color: var(--danger); color: var(--danger); background: transparent; }
  .btn.danger:hover { background: var(--danger-dim); }

  .channel-badge {
    font-size: 13px;
    color: var(--accent);
    font-weight: 700;
    padding: 6px 14px;
    background: var(--accent-dim);
    border-radius: var(--radius-sm);
    border: 1px solid rgba(124, 197, 209, 0.4);
  }
  .connected-badge {
    font-size: 12px;
    color: var(--success);
    font-weight: 600;
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .connected-badge::before {
    content: '';
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--success);
    box-shadow: 0 0 6px var(--success);
  }

  .device-code-box {
    margin-top: 16px;
    padding: 20px;
    background: var(--bg-secondary);
    border: 1px solid var(--accent);
    border-radius: var(--radius);
    text-align: center;
  }
  .device-code-hint {
    font-size: 12px;
    color: var(--text-secondary);
    margin-bottom: 10px;
  }
  .device-code {
    font-size: 36px;
    font-weight: 800;
    font-family: 'Consolas', monospace;
    color: var(--accent);
    letter-spacing: 8px;
    padding: 8px;
  }
  .device-code-url {
    font-size: 11px;
    margin-top: 10px;
  }
  .device-code-url a {
    color: var(--accent);
    text-decoration: underline;
    opacity: 0.8;
  }
  .device-code-url a:hover { opacity: 1; }

  .status-bar {
    padding: 10px 14px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    font-size: 12px;
    color: var(--text-secondary);
    margin-bottom: 16px;
  }

  .log-section { margin-top: 8px; }
  .log-section h3 { margin-bottom: 8px; }
  .log-entries {
    max-height: 200px;
    overflow-y: auto;
    font-size: 11px;
    font-family: 'Consolas', monospace;
    background: var(--bg-secondary);
    padding: 10px;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
  }
  .log-entry { padding: 3px 0; color: var(--text-secondary); }
  .log-time { color: var(--text-muted); margin-right: 8px; }

  .warn-box {
    padding: 14px 16px;
    margin-bottom: 16px;
    background: rgba(214, 163, 74, 0.10);
    border: 1px solid rgba(214, 163, 74, 0.45);
    border-radius: var(--radius);
  }
  .warn-box strong { color: var(--warning); font-size: 13px; }
  .warn-box p { font-size: 12px; color: var(--text-secondary); margin-top: 6px; line-height: 1.6; }
  .warn-hint { color: var(--text-muted) !important; }

  .linklike {
    background: none; border: none; padding: 0;
    color: var(--accent); cursor: pointer;
    font: inherit; text-decoration: underline;
  }

  .disclosure {
    background: none; border: none; padding: 0;
    color: var(--text-primary); cursor: pointer;
    font-family: var(--font-display);
    font-size: 13px; font-weight: 600; letter-spacing: 0.08em;
    text-transform: uppercase;
  }
  .app-state {
    margin-left: 8px;
    font-size: 10px; letter-spacing: 0.12em;
    color: var(--text-muted); text-transform: none;
  }

  .field { display: block; margin-top: 12px; }
  .field span {
    display: block; font-size: 11px; color: var(--text-secondary); margin-bottom: 4px;
  }
  .field span em { color: var(--text-muted); font-style: normal; }
  .field input {
    width: 100%; padding: 8px 12px;
    background: var(--bg-input); border: 1px solid var(--border);
    color: var(--text-primary); border-radius: var(--radius-sm);
    font-family: var(--font-mono); font-size: 12px;
  }
  .field input:focus {
    outline: none; border-color: rgba(124, 197, 209, 0.5);
  }

  .hint { font-size: 12px; color: var(--text-secondary); margin-top: 10px; line-height: 1.6; }
  .hint.muted { color: var(--text-muted); font-style: italic; }
  .hint code {
    font-family: var(--font-mono); font-size: 11px;
    background: var(--bg-input); padding: 1px 5px; border-radius: 2px;
  }
  .app-status { margin-top: 10px; font-size: 12px; color: var(--success); font-weight: 600; }
  .app-status.error { color: var(--danger); }
</style>