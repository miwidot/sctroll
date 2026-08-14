<script>
  import { onMount } from 'svelte'
  import { GetConfig, SetTargetWindow, GetLanguage, SetLanguage,
           GetSCDir, GetSCChannel, BrowseSCDir, IsGameRunning,
           GetBindStatus, ImportSCBinds, RemoveSCBinds,
           GetInputMode, SetInputMode, GetDebugLogPath,
           GetVersion, CheckForUpdate,
           GetAutostart, SetAutostart } from '../../wailsjs/go/main/App'
  import { i18n, locale } from '../i18n'

  let targetWindow = ''
  let currentLang = 'de'
  let scDir = ''
  let scChannel = ''
  let gameRunning = false
  let binds = []
  let bindError = ''
  let importStatus = ''
  let importing = false
  let inputMode = 'scancode'
  let logPath = ''
  let appVersion = ''
  let autostartOn = false
  let autostartStatus = ''
  let checking = false
  let updateStatus = ''

  const de = (a, b) => currentLang === 'de' ? a : b


  async function toggleAutostart(e) {
    const want = e.target.checked
    autostartStatus = ''
    try {
      await SetAutostart(want)
      autostartOn = await GetAutostart()
    } catch (err) {
      autostartStatus = de('Fehler: ', 'Error: ') + err
      autostartOn = await GetAutostart()   // Zustand zurückholen, nicht raten
    }
  }
  async function checkUpdate() {
    checking = true
    updateStatus = ''
    try {
      const r = await CheckForUpdate()
      updateStatus = r.newer
        ? de(`Version ${r.version} verfügbar — der Hinweis oben führt zum Update.`,
             `Version ${r.version} available — the banner above installs it.`)
        : de(`Du hast die neueste Version (${appVersion}).`,
             `You are on the latest version (${appVersion}).`)
    } catch (e) {
      updateStatus = de('Fehler: ', 'Error: ') + e
    }
    checking = false
  }

  async function changeInputMode(mode) {
    inputMode = mode
    await SetInputMode(mode)
  }

  onMount(async () => {
    const cfg = await GetConfig()
    targetWindow = cfg.target_window || ''
    currentLang = cfg.language || 'de'
    locale.set(currentLang)
    try {
      inputMode = await GetInputMode()
      logPath = await GetDebugLogPath()
      appVersion = await GetVersion()
      autostartOn = await GetAutostart()
    } catch(e) {}
    await refreshSC()
  })

  async function refreshSC() {
    try {
      scDir = await GetSCDir()
      scChannel = await GetSCChannel()
      gameRunning = await IsGameRunning()
    } catch(e) {}
    await loadBinds()
  }

  async function loadBinds() {
    bindError = ''
    try {
      binds = await GetBindStatus() || []
    } catch(e) {
      binds = []
      bindError = String(e)
    }
  }

  async function chooseDir() {
    try {
      const dir = await BrowseSCDir()
      if (dir) await refreshSC()
    } catch(e) {
      bindError = String(e)
    }
  }

  async function importBinds() {
    importing = true
    importStatus = ''
    try {
      const count = await ImportSCBinds()
      importStatus = count > 0
        ? de(`${count} Belegung(en) aus dem Spiel übernommen.`,
             `${count} bind(s) taken over from the game.`)
        : de('Nichts zu übernehmen — alle Tasten stimmen bereits mit deinem Profil überein.',
             'Nothing to import — all keys already match your profile.')
      await loadBinds()
    } catch(e) {
      importStatus = de('Fehler: ', 'Error: ') + e
    }
    importing = false
  }

  async function removeBinds() {
    importing = true
    importStatus = ''
    try {
      const count = await RemoveSCBinds()
      importStatus = count > 0
        ? de(`${count} von sctroll gesetzte Belegung(en) entfernt — die Standardbelegung gilt wieder.`,
             `${count} sctroll-written bind(s) removed — the default binding applies again.`)
        : de('Es stehen keine von sctroll gesetzten Belegungen in deinem Profil.',
             'No sctroll-written binds found in your profile.')
      await loadBinds()
    } catch(e) {
      importStatus = de('Fehler: ', 'Error: ') + e
    }
    importing = false
  }

  async function saveTargetWindow() {
    await SetTargetWindow(targetWindow)
  }

  async function changeLang(lang) {
    currentLang = lang
    locale.set(lang)
    await SetLanguage(lang)
  }

  // Bewusst alle Aktionen, nicht nur die aktiven: gerade die abgeschalteten
  // (Emotes, Türen) sind die, bei denen man die Belegung nachsehen will.
  $: mismatched = binds.filter(b => b.mismatch)
  $: unbound = binds.filter(b => b.source === 'unbelegt')
  $: fromProfile = binds.filter(b => b.source === 'profil')
</script>

<div class="panel">
  <h2>{$i18n('settings.title')}</h2>

  <div class="section chamfer">
    <h3>{$i18n('settings.language')}</h3>
    <div class="lang-selector">
      <button class="lang-btn" class:active={currentLang === 'de'} on:click={() => changeLang('de')}>
        Deutsch
      </button>
      <button class="lang-btn" class:active={currentLang === 'en'} on:click={() => changeLang('en')}>
        English
      </button>
    </div>
  </div>

  <div class="section sc-section chamfer">
    <h3>{de('Star-Citizen-Installation', 'Star Citizen installation')}</h3>
    <p class="hint">
      {de('Wird automatisch gesucht (RSI-Launcher-Log und alle lokalen Laufwerke). Falls das Spiel woanders liegt, Ordner manuell wählen.',
          'Detected automatically (RSI launcher log and all local drives). If the game lives elsewhere, pick the folder manually.')}
    </p>
    {#if scDir}
      <p class="path"><span class="channel-badge">{scChannel}</span> {scDir}</p>
    {:else}
      <p class="import-status error">{de('Nicht gefunden — bitte Ordner wählen.', 'Not found — please choose the folder.')}</p>
    {/if}
    <button class="btn" on:click={chooseDir}>{de('Ordner wählen…', 'Choose folder…')}</button>
  </div>

  <div class="section sc-section chamfer">
    <h3>{de('Tastenbelegungen', 'Key bindings')}</h3>
    <p class="hint">
      {de('sctroll liest deine actionmaps.xml und übernimmt daraus die Tasten. Geschrieben wird nichts — ein Eintrag in der Datei würde Star Citizens Standardbelegung überschreiben.',
          'sctroll reads your actionmaps.xml and takes the keys from there. Nothing is written — an entry in that file would override Star Citizen\'s default binding.')}
    </p>
    <p class="hint muted">
      {de('Die gültige Taste kommt aus zwei Quellen: „Eigene“ steht in deiner actionmaps.xml, „Standard“ aus Star Citizens defaultProfile.xml. Eine Joystick-Belegung ersetzt die Tastatur nicht — beide gelten nebeneinander. „Unbelegt“ heißt, die Aktion hat im Spiel gar keine Taste; die musst du dort selbst binden.',
          'The effective key comes from two sources: “custom” lives in your actionmaps.xml, “default” comes from Star Citizen\'s defaultProfile.xml. A joystick binding does not replace the keyboard — both apply side by side. “Unbound” means the action has no key in the game at all; bind it there yourself.')}
    </p>

    {#if bindError}
      <p class="import-status error">{bindError}</p>
    {:else if binds.length}
      <div class="bind-list">
        {#each binds as b}
          <div class="bind-row" class:warn={b.mismatch} class:unbound={b.source === 'unbelegt'} class:off={!b.enabled}>
            <span class="bind-name">{b.name}</span>
            <span class="bind-action">{b.sc_action}</span>
            <span class="bind-key">{b.effective || b.key}</span>
            <span class="bind-src" class:profile={b.source === 'profil'} class:none={b.source === 'unbelegt'}>
              {b.source === 'profil' ? de('Eigene', 'custom')
                : b.source === 'standard' ? de('Standard', 'default')
                : de('unbelegt', 'unbound')}
            </span>
          </div>
        {/each}
      </div>
      {#if mismatched.length}
        <p class="import-status error">
          {de(`${mismatched.length} Aktion(en) drücken etwas anderes, als im Spiel gilt — „Aus dem Spiel übernehmen“ korrigiert das.`,
              `${mismatched.length} action(s) press something other than what the game uses — “Import from game” fixes that.`)}
        </p>
      {/if}
      {#if unbound.length}
        <p class="import-status error">
          {de(`${unbound.length} Aktion(en) haben im Spiel gar keine Taste. Die musst du in Star Citizen selbst binden — Türen und Emotes sind ab Werk unbelegt.`,
              `${unbound.length} action(s) have no key in the game at all. Bind them in Star Citizen yourself — doors and emotes are unbound out of the box.`)}
        </p>
      {/if}
      <p class="hint muted">
        {de(`${fromProfile.length} eigene Belegung, ${binds.length - fromProfile.length - unbound.length} Standardbelegung, ${unbound.length} unbelegt. Ausgegraute Zeilen sind abgeschaltete Aktionen.`,
            `${fromProfile.length} custom, ${binds.length - fromProfile.length - unbound.length} default, ${unbound.length} unbound. Greyed rows are disabled actions.`)}
      </p>
    {/if}

    <button class="btn import-btn" on:click={importBinds} disabled={importing || !scDir}>
      {importing ? de('Lese…', 'Reading…') : de('Aus dem Spiel übernehmen', 'Import from game')}
    </button>
    <button class="btn" on:click={refreshSC}>{de('Neu prüfen', 'Re-check')}</button>

    <p class="hint muted cleanup-hint">
      {de('Frühere sctroll-Versionen haben RAlt-Belegungen in die actionmaps.xml geschrieben und damit die Standardtasten überschrieben. Der Knopf nimmt genau die wieder zurück.',
          'Earlier sctroll versions wrote RAlt bindings into actionmaps.xml, overriding the stock keys. This button takes exactly those back out.')}
    </p>
    <button class="btn danger-btn" on:click={removeBinds} disabled={importing || gameRunning || !scDir}>
      {de('Von sctroll gesetzte Belegungen entfernen', 'Remove sctroll-written bindings')}
    </button>
    {#if gameRunning}
      <p class="import-status error">
        {de('Star Citizen läuft — zum Entfernen bitte erst beenden.', 'Star Citizen is running — quit it before removing.')}
      </p>
    {/if}

    {#if importStatus}
      <p class="import-status" class:error={importStatus.startsWith('Fehler') || importStatus.startsWith('Error')}>
        {importStatus}
      </p>
    {/if}
  </div>

  <div class="section sc-section chamfer">
    <h3>{de('Sendeverfahren', 'Input method')}</h3>
    <p class="hint">
      {de('Wie der Tastendruck ans Spiel geschickt wird. Welches Verfahren Star Citizen annimmt, lässt sich nicht vorhersagen — probier es mit dem Test-Knopf bei einer Aktion durch. Das Debug-Log zeigt danach, ob der Druck angenommen oder blockiert wurde.',
          'How the key press is delivered to the game. Which method Star Citizen accepts cannot be predicted — try them with the Test button on an action. The debug log then shows whether the press was accepted or blocked.')}
    </p>
    <div class="lang-selector">
      {#each [['scancode','Scancode'],['virtual','Virtual-Key'],['both',de('Beides','Both')]] as [id, label]}
        <button class="lang-btn" class:active={inputMode === id} on:click={() => changeInputMode(id)}>{label}</button>
      {/each}
    </div>
    <p class="hint muted" style="margin-top:12px">
      {de('Scancode ist der Standard. Virtual-Key ist das, was InputSimulator macht — damit funktioniert das Star-Citizen-Stream-Deck-Plugin nachweislich. Läuft Star Citizen als Administrator, muss SCTroll das auch, sonst blockt Windows jeden Druck.',
          'Scancode is the default. Virtual-Key is what InputSimulator does — the Star Citizen Stream Deck plugin demonstrably works with it. If Star Citizen runs as administrator, SCTroll has to as well, otherwise Windows blocks every press.')}
    </p>
    <p class="path">{de('Debug-Log', 'Debug log')}: {logPath}</p>
  </div>

  <div class="section chamfer">
    <h3>{de('Mit Windows starten', 'Start with Windows')}</h3>
    <p class="hint">
      {de('Trägt SCTroll in den Autostart deines Benutzerkontos ein. Keine Administratorrechte nötig, und nach einem Update zeigt der Eintrag automatisch auf die neue Programmdatei.',
          'Adds SCTroll to your user account\'s startup. No administrator rights needed, and after an update the entry automatically points at the new executable.')}
    </p>
    <label class="switch-row">
      <input type="checkbox" checked={autostartOn} on:change={toggleAutostart} />
      <span>{autostartOn ? de('Startet mit Windows', 'Starts with Windows') : de('Startet nicht mit Windows', 'Does not start with Windows')}</span>
    </label>
    {#if autostartStatus}
      <p class="import-status" class:error={autostartStatus.startsWith('Fehler')}>{autostartStatus}</p>
    {/if}
  </div>

  <div class="section chamfer">
    <h3>{$i18n('settings.target_window')}</h3>
    <p class="hint">{$i18n('settings.target_window.hint')}</p>
    <div class="form-row">
      <input type="text" bind:value={targetWindow} placeholder="StarCitizen" />
      <button class="btn" on:click={saveTargetWindow}>{$i18n('actions.save')}</button>
    </div>
  </div>

  <div class="section chamfer">
    <h3>{$i18n('settings.keylock.title')}</h3>
    <p class="hint">{$i18n('settings.keylock.desc')}</p>
    <p class="hint muted">{$i18n('settings.keylock.hint')}</p>
  </div>

  <div class="section about chamfer">
    <h3>{$i18n('settings.about')}</h3>
    <p class="hint">{$i18n('settings.about.desc')}</p>
    <div class="about-footer">
      <span class="about-brand">SCTroll</span>
      <span class="about-version">v{appVersion}</span>
    </div>

    <div class="form-row" style="margin-top:14px">
      <button class="btn" on:click={checkUpdate} disabled={checking}>
        {checking ? de('Suche…', 'Checking…') : de('Nach Updates suchen', 'Check for updates')}
      </button>
    </div>
    {#if updateStatus}
      <p class="import-status" class:error={updateStatus.startsWith('Fehler')}>{updateStatus}</p>
    {/if}
    <p class="hint muted" style="margin-top:10px">
      {de('Beim Start wird still im Hintergrund geprüft. Eingespielt wird nur auf Knopfdruck — ein Neustart mitten im Stream wäre ein schlechter Zeitpunkt. Vor dem Austausch werden Prüfsumme und Signatur kontrolliert.',
          'A silent check runs at startup. Installing only ever happens on click — a restart mid-stream would be a bad moment. Checksum and signature are verified before the swap.')}
    </p>
  </div>
</div>

<style>
  .panel { max-width: 640px; }
  h2 {
    font-size: 20px;
    font-weight: 700;
    color: var(--ink);
    letter-spacing: 1px;
    margin-bottom: 22px;
  }
  h3 {
    font-size: 14px;
    color: var(--text-primary);
    font-weight: 600;
    margin-bottom: 10px;
  }

  .section {
    margin-bottom: 16px;
    padding: 18px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }

  .hint {
    font-size: 12px;
    color: var(--text-secondary);
    margin-bottom: 12px;
    line-height: 1.6;
  }
  .hint.muted {
    color: var(--text-muted);
    font-style: italic;
    margin-bottom: 0;
  }

  .lang-selector {
    display: flex;
    gap: 8px;
  }
  .lang-btn {
    padding: 8px 24px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    cursor: pointer;
    border-radius: var(--radius-sm);
    font-size: 13px;
    font-weight: 600;
    transition: all 0.15s;
  }
  .lang-btn.active {
    background: rgba(124, 197, 209, 0.10);
    border-color: rgba(124, 197, 209, 0.40);
    color: var(--accent);
  }
  .lang-btn:hover:not(.active) {
    border-color: var(--border-hover);
    color: var(--text-primary);
  }

  .form-row {
    display: flex;
    gap: 10px;
  }
  .form-row input {
    flex: 1;
    padding: 8px 12px;
    background: var(--bg-input);
    border: 1px solid var(--border);
    color: var(--text-primary);
    border-radius: var(--radius-sm);
    font-size: 13px;
  }
  .form-row input:focus {
    outline: none;
    border-color: rgba(124, 197, 209, 0.50);
    box-shadow: 0 0 0 2px rgba(124, 197, 209, 0.12);
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
  .btn:hover { border-color: rgba(124, 197, 209, 0.40); color: var(--accent); }

  .sc-section {
    border-color: rgba(124, 197, 209, 0.4);
    background: linear-gradient(135deg, var(--bg-card), rgba(124, 197, 209, 0.08));
  }

  .channel-badge {
    display: inline-block;
    padding: 1px 7px;
    margin-right: 8px;
    border-radius: var(--radius-sm);
    background: rgba(124, 197, 209, 0.12);
    color: var(--accent);
    font-weight: 700;
    letter-spacing: 1px;
  }

  .bind-list {
    max-height: 220px;
    overflow-y: auto;
    margin-bottom: 12px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
  }
  .bind-row {
    display: grid;
    grid-template-columns: 1fr 1.3fr auto 62px;
    gap: 10px;
    align-items: center;
    padding: 5px 10px;
    font-size: 11px;
    border-bottom: 1px solid var(--border);
  }
  .bind-row:last-child { border-bottom: none; }
  .bind-name { color: var(--text-primary); }
  .bind-action {
    color: var(--text-muted);
    font-family: 'Consolas', monospace;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .bind-key {
    font-family: 'Consolas', monospace;
    color: var(--text-secondary);
    text-transform: uppercase;
  }
  .bind-src {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.10em;
    color: var(--text-muted);
    text-align: right;
  }
  .bind-src.profile { color: var(--success); }
  .bind-src.none { color: var(--warning); }
  .bind-row.warn { background: rgba(239, 84, 84, 0.07); }
  .bind-row.warn .bind-key { color: var(--danger); }
  .bind-row.off { opacity: 0.45; }
  .bind-row.unbound { background: rgba(214, 163, 74, 0.07); }
  .bind-row.unbound .bind-key { color: var(--warning); text-decoration: line-through; }

  .cleanup-hint { margin-top: 18px; }
  .danger-btn {
    border-color: rgba(239, 84, 84, 0.45);
    color: var(--danger);
  }
  .danger-btn:hover { border-color: var(--danger); color: var(--alert-bright); }
  .danger-btn:disabled { opacity: 0.4; cursor: not-allowed; }

  .import-btn {
    background: var(--accent) !important;
    color: var(--ink) !important;
    border-color: var(--accent) !important;
    font-weight: 700;
    padding: 10px 24px;
    font-size: 13px;
    margin-right: 8px;
  }
  .import-btn:hover { filter: brightness(1.1); }
  .import-btn:disabled { opacity: 0.45; cursor: not-allowed; }

  .path {
    font-size: 11px;
    color: var(--text-muted);
    font-family: 'Consolas', monospace;
    margin-bottom: 12px;
    word-break: break-all;
    opacity: 0.7;
  }

  .import-status {
    margin-top: 12px;
    font-size: 13px;
    color: var(--success);
    font-weight: 600;
  }
  .import-status.error { color: var(--danger); }

  .about {
    border-color: var(--border);
  }
  .about-footer {
    display: flex;
    align-items: baseline;
    gap: 10px;
    margin-top: 8px;
  }
  .about-brand {
    font-size: 14px;
    font-weight: 800;
    color: var(--accent);
    letter-spacing: 3px;
  }
  .about-version {
    font-size: 11px;
    color: var(--text-muted);
  }

  .switch-row { display: flex; align-items: center; gap: 10px; cursor: pointer; }
  .switch-row input { width: 16px; height: 16px; accent-color: var(--accent); cursor: pointer; }
  .switch-row span { font-size: 13px; color: var(--text-primary); }
</style>