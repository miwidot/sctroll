<script>
  import { onMount, onDestroy } from 'svelte'
  import { GetActions, ToggleAction, UpdateAction, AddCustomAction, DeleteAction, GetGlobalEnable, SetGlobalEnable, TestAction } from '../../wailsjs/go/main/App'
  import { EventsOn } from '../../wailsjs/runtime/runtime'
  import { i18n } from '../i18n'

  let actions = []
  let editingAction = null
  let creatingAction = false
  let newAction = getEmptyAction()
  let filterCategory = 'all'
  let globalEnabled = true
  let unsubscribers = []
  let deleteConfirm = false

  // Test: 5 Sekunden Vorlauf, damit man ins Spiel wechseln kann. Der
  // Tastendruck geht nur an das Fenster im Vordergrund.
  const TEST_DELAY = 5
  let testing = null
  let countdown = 0

  function testAction(action) {
    testing = action.id
    countdown = TEST_DELAY
    TestAction(action.id, TEST_DELAY)
  }

  function getEmptyAction() {
    return {
      name: '',
      description: '',
      reward_title: '',
      reward_cost: 100,
      key: '',
      hold_ms: 100,
      repeat: 0,
      repeat_delay_ms: 0,
      category: 'custom',
      key_lock: { enabled: false, keys: [], duration_ms: 0 },
      cooldown_ms: 10000,
      twitch_cooldown_sec: 60,
      max_per_stream: 0,
      max_per_user_per_stream: 0,
      reward_color: '#9146FF',
    }
  }

  $: categories = [
    { id: 'all', label: $i18n('actions.cat.all') },
    { id: 'ship', label: $i18n('actions.cat.ship') },
    { id: 'flight', label: $i18n('actions.cat.flight') },
    { id: 'power', label: $i18n('actions.cat.power') },
    { id: 'view', label: $i18n('actions.cat.view') },
    { id: 'player', label: $i18n('actions.cat.player') },
    { id: 'emote', label: $i18n('actions.cat.emote') },
    { id: 'danger', label: $i18n('actions.cat.danger') },
    { id: 'fun', label: $i18n('actions.cat.fun') },
    { id: 'custom', label: $i18n('actions.cat.custom') },
  ]

  onMount(async () => {
    await loadActions()
    globalEnabled = await GetGlobalEnable()

    unsubscribers.push(EventsOn('action-executed', (data) => {
      const idx = actions.findIndex(a => a.id === data.action)
      if (idx >= 0) {
        actions[idx]._flash = true
        actions = actions
        setTimeout(() => {
          actions[idx]._flash = false
          actions = actions
        }, 1000)
      }
    }))
    unsubscribers.push(EventsOn('global-toggle', (val) => {
      globalEnabled = val
    }))
    unsubscribers.push(EventsOn('actions-updated', async () => {
      await loadActions()
    }))
    unsubscribers.push(EventsOn('test-countdown', (d) => {
      if (d.action === testing) countdown = d.left
    }))
    unsubscribers.push(EventsOn('test-result', (d) => {
      if (d.action === testing) {
        testing = null
        countdown = 0
      }
    }))
  })

  onDestroy(() => unsubscribers.forEach(fn => fn()))

  async function loadActions() {
    actions = await GetActions()
  }

  async function toggle(action) {
    // Neu laden in jedem Fall: scheitert der Aufruf, bliebe der Schalter sonst
    // im alten Zustand stehen und man hielte ihn für kaputt.
    try {
      await ToggleAction(action.id, !action.enabled)
    } catch (e) {
      console.error('ToggleAction:', e)
    } finally {
      await loadActions()
    }
  }

  async function enableAll() {
    for (const action of actions) {
      if (!action.enabled) {
        await ToggleAction(action.id, true)
      }
    }
    await loadActions()
  }

  async function disableAll() {
    for (const action of actions) {
      if (action.enabled) {
        await ToggleAction(action.id, false)
      }
    }
    await loadActions()
  }

  async function toggleGlobal() {
    globalEnabled = !globalEnabled
    await SetGlobalEnable(globalEnabled)
  }

  function startEdit(action) {
    editingAction = JSON.parse(JSON.stringify(action))
  }

  async function saveEdit() {
    await UpdateAction(editingAction)
    editingAction = null
    await loadActions()
  }

  function cancelEdit() {
    editingAction = null
    deleteConfirm = false
  }

  function startCreate() {
    newAction = getEmptyAction()
    creatingAction = true
  }

  async function saveCreate() {
    if (!newAction.name || !newAction.key || !newAction.reward_title) return
    await AddCustomAction(newAction)
    creatingAction = false
    await loadActions()
  }

  function cancelCreate() {
    creatingAction = false
  }

  async function deleteCustomAction() {
    if (!editingAction || !editingAction.custom) return
    if (!deleteConfirm) {
      deleteConfirm = true
      return
    }
    await DeleteAction(editingAction.id)
    editingAction = null
    deleteConfirm = false
    await loadActions()
  }

  $: filteredActions = filterCategory === 'all'
    ? actions
    : actions.filter(a => a.category === filterCategory)

  $: enabledCount = actions.filter(a => a.enabled).length
  $: totalCount = actions.length
</script>

<div class="panel" class:global-paused={!globalEnabled}>
  {#if !globalEnabled}
    <div class="paused-banner" on:click={toggleGlobal}>
      <div class="paused-left">
        <span class="paused-icon">||</span>
        <div class="paused-text">
          <strong>{$i18n('actions.paused')}</strong>
          <span>{$i18n('actions.paused_hint')}</span>
        </div>
      </div>
      <button class="resume-btn">{$i18n('app.active')}</button>
    </div>
  {/if}

  <div class="panel-header">
    <div class="panel-header-left">
      <h2>{$i18n('actions.title')}</h2>
      <span class="action-count">{enabledCount}/{totalCount}</span>
    </div>
    <div class="panel-header-right">
      <button class="add-btn" on:click={startCreate} title={$i18n('actions.add_custom')}>+</button>
      <div class="bulk-actions">
        <button class="bulk-btn enable" on:click={enableAll}>{$i18n('actions.enable_all')}</button>
        <button class="bulk-btn disable" on:click={disableAll}>{$i18n('actions.disable_all')}</button>
      </div>
      <div class="category-filter">
        {#each categories as cat}
          <button
            class="filter-btn"
            class:active={filterCategory === cat.id}
            on:click={() => filterCategory = cat.id}
          >{cat.label}</button>
        {/each}
      </div>
    </div>
  </div>

  <div class="actions-grid">
    {#each filteredActions as action}
      <div class="action-card" class:disabled={!action.enabled} class:flash={action._flash}>
        <div class="action-header">
          <div class="action-title-row">
            <span class="action-name">
              {action.name}
              {#if action.custom}<span class="custom-badge">CUSTOM</span>{/if}
            </span>
            <label class="toggle-switch">
              <input type="checkbox" checked={action.enabled} on:change={() => toggle(action)} />
              <span class="toggle-slider"></span>
            </label>
          </div>
          <div class="action-desc-row">
            <span class="action-desc">{action.description}</span>
            <span class="info-icon" title="">
              i
              <div class="tooltip">
                <strong>{action.name}</strong>
                <p>{action.description}</p>
                <div class="tooltip-details">
                  <span>Key: <em>{action.key.toUpperCase()}</em></span>
                  <span>Cost: <em>{action.reward_cost}</em></span>
                  <span>Cooldown: <em>{action.cooldown_ms / 1000}s</em></span>
                  {#if action.steps && action.steps.length > 0}
                    <span>Steps: <em>{action.steps.length}</em></span>
                  {/if}
                  {#if action.max_per_stream > 0 || action.max_per_user_per_stream > 0}
            <div class="detail">
              <span class="detail-label">Limit</span>
              <span class="detail-value limit-badge">
                {action.max_per_stream > 0 ? action.max_per_stream + "/Stream" : ""}{action.max_per_stream > 0 && action.max_per_user_per_stream > 0 ? " · " : ""}{action.max_per_user_per_stream > 0 ? action.max_per_user_per_stream + "/Zuschauer" : ""}
              </span>
            </div>
          {/if}
          {#if action.key_lock.enabled}
                    <span>Lock: <em>{action.key_lock.keys.join(', ').toUpperCase()} ({action.key_lock.duration_ms / 1000}s)</em></span>
                  {/if}
                </div>
              </div>
            </span>
          </div>
        </div>
        <div class="action-details">
          <div class="detail">
            <span class="detail-label">{$i18n('actions.key')}</span>
            <span class="detail-value key-badge">{action.key.toUpperCase()}</span>
          </div>
          <div class="detail">
            <span class="detail-label">{$i18n('actions.cost')}</span>
            <span class="detail-value cost-value">{action.reward_cost}</span>
          </div>
          <div class="detail">
            <span class="detail-label">Cooldown</span>
            <span class="detail-value">{action.cooldown_ms / 1000}s</span>
          </div>
          <div class="detail">
            <span class="detail-label">Twitch CD</span>
            <span class="detail-value twitch-cd">{action.twitch_cooldown_sec ? action.twitch_cooldown_sec + 's' : Math.max(60, action.cooldown_ms / 1000) + 's'}</span>
          </div>
          {#if action.key_lock.enabled}
            <div class="detail lock-detail">
              <span class="detail-label">{$i18n('actions.keylock')}</span>
              <span class="detail-value lock-badge">{action.key_lock.keys.join(', ').toUpperCase()} ({action.key_lock.duration_ms / 1000}s)</span>
            </div>
          {/if}
        </div>
        <div class="card-buttons">
          <button class="edit-btn" on:click={() => startEdit(action)}>{$i18n('actions.edit')}</button>
          <button class="edit-btn test-btn" on:click={() => testAction(action)} disabled={testing === action.id}>
            {#if testing === action.id}
              {countdown > 0 ? `${countdown}…` : '▶'}
            {:else}
              {$i18n('actions.test')}
            {/if}
          </button>
        </div>
      </div>
    {/each}
  </div>
</div>

{#if editingAction}
  <div class="modal-overlay" on:click|self={cancelEdit}>
    <div class="modal">
      <div class="modal-header">
        <h3>{$i18n('actions.edit_title')}</h3>
        <span class="modal-subtitle">{editingAction.name}</span>
      </div>

      <div class="form-grid">
        <label>
          <span>{$i18n('actions.name')}</span>
          <input type="text" bind:value={editingAction.name} />
        </label>
        <label>
          <span>{$i18n('actions.description')}</span>
          <input type="text" bind:value={editingAction.description} />
        </label>
        <label>
          <span>{$i18n('actions.key_hint')}</span>
          <input type="text" bind:value={editingAction.key} />
        </label>
        <label>
          <span>{$i18n('actions.hold_ms')}</span>
          <input type="number" bind:value={editingAction.hold_ms} />
        </label>
        <label>
          <span>{$i18n('actions.repeat')}</span>
          <input type="number" bind:value={editingAction.repeat} min="0" placeholder="1" />
        </label>
        <label>
          <span>{$i18n('actions.repeat_delay')}</span>
          <input type="number" bind:value={editingAction.repeat_delay_ms} min="0" placeholder="100" />
        </label>
        <label>
          <span>{$i18n('actions.reward_title')}</span>
          <input type="text" bind:value={editingAction.reward_title} />
        </label>
        <label>
          <span>{$i18n('actions.reward_cost')}</span>
          <input type="number" bind:value={editingAction.reward_cost} />
        </label>
        <label>
          <span>Lokaler Cooldown (Sek)</span>
          <input type="number" min="0" step="1"
            value={editingAction.cooldown_ms / 1000}
            on:input={(e) => editingAction.cooldown_ms = Math.round(e.target.value * 1000)} />
        </label>
        <label>
          <span>Twitch Cooldown (Sek, min 60)</span>
          <input type="number" min="60" step="1"
            bind:value={editingAction.twitch_cooldown_sec}
            placeholder="60" />
        </label>
        <label>
          <span>Max. pro Stream (0 = unbegrenzt)</span>
          <input type="number" min="0" step="1" bind:value={editingAction.max_per_stream} placeholder="0" />
        </label>
        <label>
          <span>Max. pro Zuschauer und Stream (0 = unbegrenzt)</span>
          <input type="number" min="0" step="1" bind:value={editingAction.max_per_user_per_stream} placeholder="0" />
        </label>
        <label>
          <span>{$i18n('actions.category')}</span>
          <select bind:value={editingAction.category}>
            <option value="ship">{$i18n('actions.cat.ship')}</option>
            <option value="flight">{$i18n('actions.cat.flight')}</option>
            <option value="power">{$i18n('actions.cat.power')}</option>
            <option value="view">{$i18n('actions.cat.view')}</option>
            <option value="player">{$i18n('actions.cat.player')}</option>
            <option value="emote">{$i18n('actions.cat.emote')}</option>
            <option value="danger">{$i18n('actions.cat.danger')}</option>
            <option value="fun">{$i18n('actions.cat.fun')}</option>
            <option value="custom">{$i18n('actions.cat.custom')}</option>
          </select>
        </label>
        <label>
          <span>Reward Farbe</span>
          <div class="color-row">
            <input type="color" bind:value={editingAction.reward_color} class="color-picker" />
            <input type="text" bind:value={editingAction.reward_color} placeholder="#9146FF" class="color-text" />
          </div>
        </label>
      </div>

      <div class="keylock-section">
        <label class="checkbox-label">
          <input type="checkbox" bind:checked={editingAction.key_lock.enabled} />
          <span>{$i18n('actions.keylock_enable')}</span>
        </label>
        {#if editingAction.key_lock.enabled}
          <label>
            <span>{$i18n('actions.locked_keys')}</span>
            <input type="text" value={editingAction.key_lock.keys.join(', ')}
              on:input={(e) => editingAction.key_lock.keys = e.target.value.split(',').map(k => k.trim()).filter(Boolean)} />
          </label>
          <label>
            <span>{$i18n('actions.lock_duration')}</span>
            <input type="number" bind:value={editingAction.key_lock.duration_ms} />
          </label>
        {/if}
      </div>

      <div class="modal-buttons">
        {#if editingAction.custom}
          <button class="btn-delete" on:click={deleteCustomAction}>
            {deleteConfirm ? $i18n('actions.delete_confirm') : $i18n('actions.delete')}
          </button>
        {/if}
        <div style="flex:1"></div>
        <button class="btn-cancel" on:click={cancelEdit}>{$i18n('actions.cancel')}</button>
        <button class="btn-save" on:click={saveEdit}>{$i18n('actions.save')}</button>
      </div>
    </div>
  </div>
{/if}

{#if creatingAction}
  <div class="modal-overlay" on:click|self={cancelCreate}>
    <div class="modal">
      <div class="modal-header">
        <h3>{$i18n('actions.create_title')}</h3>
      </div>

      <div class="form-grid">
        <label>
          <span>{$i18n('actions.name')}</span>
          <input type="text" bind:value={newAction.name} placeholder="Meine Aktion" />
        </label>
        <label>
          <span>{$i18n('actions.description')}</span>
          <input type="text" bind:value={newAction.description} placeholder="Was macht die Aktion?" />
        </label>
        <label>
          <span>{$i18n('actions.key_hint')}</span>
          <input type="text" bind:value={newAction.key} placeholder="g, space, alt+t..." />
        </label>
        <label>
          <span>{$i18n('actions.hold_ms')}</span>
          <input type="number" bind:value={newAction.hold_ms} />
        </label>
        <label>
          <span>{$i18n('actions.repeat')}</span>
          <input type="number" bind:value={newAction.repeat} min="0" placeholder="0" />
        </label>
        <label>
          <span>{$i18n('actions.repeat_delay')}</span>
          <input type="number" bind:value={newAction.repeat_delay_ms} min="0" placeholder="100" />
        </label>
        <label>
          <span>{$i18n('actions.reward_title')}</span>
          <input type="text" bind:value={newAction.reward_title} placeholder="Mein Reward!" />
        </label>
        <label>
          <span>{$i18n('actions.reward_cost')}</span>
          <input type="number" bind:value={newAction.reward_cost} />
        </label>
        <label>
          <span>Lokaler Cooldown (Sek)</span>
          <input type="number" min="0" step="1"
            value={newAction.cooldown_ms / 1000}
            on:input={(e) => newAction.cooldown_ms = Math.round(e.target.value * 1000)} />
        </label>
        <label>
          <span>Twitch Cooldown (Sek, min 60)</span>
          <input type="number" min="60" step="1"
            bind:value={newAction.twitch_cooldown_sec} />
        </label>
        <label>
          <span>Max. pro Stream (0 = unbegrenzt)</span>
          <input type="number" min="0" step="1" bind:value={newAction.max_per_stream} placeholder="0" />
        </label>
        <label>
          <span>Max. pro Zuschauer und Stream (0 = unbegrenzt)</span>
          <input type="number" min="0" step="1" bind:value={newAction.max_per_user_per_stream} placeholder="0" />
        </label>
        <label>
          <span>{$i18n('actions.category')}</span>
          <select bind:value={newAction.category}>
            <option value="ship">{$i18n('actions.cat.ship')}</option>
            <option value="flight">{$i18n('actions.cat.flight')}</option>
            <option value="power">{$i18n('actions.cat.power')}</option>
            <option value="view">{$i18n('actions.cat.view')}</option>
            <option value="player">{$i18n('actions.cat.player')}</option>
            <option value="emote">{$i18n('actions.cat.emote')}</option>
            <option value="danger">{$i18n('actions.cat.danger')}</option>
            <option value="fun">{$i18n('actions.cat.fun')}</option>
            <option value="custom">{$i18n('actions.cat.custom')}</option>
          </select>
        </label>
        <label>
          <span>Reward Farbe</span>
          <div class="color-row">
            <input type="color" bind:value={newAction.reward_color} class="color-picker" />
            <input type="text" bind:value={newAction.reward_color} placeholder="#9146FF" class="color-text" />
          </div>
        </label>
      </div>

      <div class="keylock-section">
        <label class="checkbox-label">
          <input type="checkbox" bind:checked={newAction.key_lock.enabled} />
          <span>{$i18n('actions.keylock_enable')}</span>
        </label>
        {#if newAction.key_lock.enabled}
          <label>
            <span>{$i18n('actions.locked_keys')}</span>
            <input type="text" value={newAction.key_lock.keys.join(', ')}
              on:input={(e) => newAction.key_lock.keys = e.target.value.split(',').map(k => k.trim()).filter(Boolean)} />
          </label>
          <label>
            <span>{$i18n('actions.lock_duration')}</span>
            <input type="number" bind:value={newAction.key_lock.duration_ms} />
          </label>
        {/if}
      </div>

      <div class="modal-buttons">
        <button class="btn-cancel" on:click={cancelCreate}>{$i18n('actions.cancel')}</button>
        <button class="btn-save" on:click={saveCreate}>{$i18n('actions.create')}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .panel { height: 100%; display: flex; flex-direction: column; }
  .panel.global-paused .actions-grid { opacity: 0.3; pointer-events: none; filter: grayscale(0.5); }

  .paused-banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 20px;
    background: var(--danger-dim);
    border: 1px solid rgba(224, 85, 85, 0.3);
    border-radius: var(--radius);
    margin-bottom: 16px;
    cursor: pointer;
    transition: all 0.2s;
  }
  .paused-banner:hover { border-color: var(--danger); }
  .paused-left { display: flex; align-items: center; gap: 14px; }
  .paused-icon {
    font-size: 20px;
    font-weight: 900;
    color: var(--danger);
    letter-spacing: -2px;
  }
  .paused-text { display: flex; flex-direction: column; gap: 2px; }
  .paused-text strong { font-size: 13px; color: var(--danger); letter-spacing: 2px; }
  .paused-text span { font-size: 11px; color: var(--text-secondary); }
  .resume-btn {
    padding: 8px 20px;
    background: var(--danger);
    border: none;
    color: white;
    font-weight: 700;
    font-size: 12px;
    letter-spacing: 1px;
    cursor: pointer;
    border-radius: var(--radius-sm);
  }
  .resume-btn:hover { filter: brightness(1.15); }

  .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 18px;
    flex-wrap: wrap;
    gap: 12px;
  }
  .panel-header-left {
    display: flex;
    align-items: baseline;
    gap: 12px;
  }
  .panel-header h2 {
    font-size: 20px;
    font-weight: 700;
    color: var(--ink);
    letter-spacing: 1px;
  }
  .action-count {
    font-size: 12px;
    color: var(--text-muted);
    background: var(--bg-card);
    padding: 3px 10px;
    border-radius: 12px;
    border: 1px solid var(--border);
    font-weight: 500;
  }

  /* Star Citizen bringt deutlich mehr Kategorien mit als die Tarkov-Vorlage.
     Die Leiste muss deshalb umbrechen, sonst laeuft sie aus dem Fenster. */
  .panel-header-right {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px;
    flex: 1;
    justify-content: flex-end;
  }

  .bulk-actions { display: flex; gap: 4px; flex-shrink: 0; }
  .bulk-btn {
    white-space: nowrap;
    padding: 5px 12px;
    background: transparent;
    border: 1px solid var(--border);
    color: var(--text-secondary);
    font-size: 11px;
    cursor: pointer;
    transition: all 0.15s;
    border-radius: var(--radius-sm);
  }
  .bulk-btn.enable:hover { border-color: var(--success); color: var(--success); }
  .bulk-btn.disable:hover { border-color: var(--danger); color: var(--danger); }

  .category-filter { display: flex; gap: 4px; flex-wrap: wrap; justify-content: flex-end; }
  .filter-btn {
    white-space: nowrap;
    padding: 5px 12px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    font-size: 12px;
    cursor: pointer;
    transition: all 0.15s;
    border-radius: var(--radius-sm);
  }
  .filter-btn:hover { border-color: var(--border-hover); color: var(--text-primary); }
  .filter-btn.active {
    background: rgba(124, 197, 209, 0.10);
    border-color: rgba(124, 197, 209, 0.40);
    color: var(--accent);
    font-weight: 600;
  }

  .actions-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
    gap: 16px;
    overflow-y: auto;
    flex: 1;
    padding-bottom: 10px;
    transition: opacity 0.3s, filter 0.3s;
  }

  .action-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 20px;
    transition: all 0.2s;
    display: flex;
    flex-direction: column;
    position: relative;
    overflow: visible;
  }
  .action-card:hover { border-color: rgba(124, 197, 209, 0.20); background: var(--bg-card-hover); }
  .action-card.disabled { opacity: 0.45; }
  .action-card.flash {
    border-color: var(--accent);
    box-shadow: 0 0 20px rgba(124, 197, 209, 0.15), inset 0 0 20px rgba(124, 197, 209, 0.08);
  }

  .action-header { margin-bottom: 14px; }
  .action-title-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 6px;
  }
  .action-name { font-weight: 700; font-size: 16px; color: var(--text-primary); }
  .action-desc-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .action-desc { font-size: 12px; color: var(--text-secondary); line-height: 1.4; flex: 1; }

  .info-icon {
    position: relative;
    width: 18px;
    height: 18px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 10px;
    font-weight: 700;
    font-style: italic;
    color: var(--text-muted);
    border: 1px solid var(--border);
    border-radius: 50%;
    cursor: help;
    flex-shrink: 0;
    transition: all 0.15s;
  }
  .info-icon:hover {
    color: var(--accent);
    border-color: rgba(124, 197, 209, 0.40);
    background: rgba(124, 197, 209, 0.08);
  }

  .tooltip {
    display: none;
    position: absolute;
    top: calc(100% + 10px);
    right: -10px;
    width: 260px;
    padding: 14px;
    background: var(--bg-secondary);
    border: 1px solid var(--border-hover);
    border-radius: var(--radius);
    box-shadow: 0 8px 30px rgba(0,0,0,0.5);
    z-index: 100;
    text-align: left;
    font-style: normal;
  }
  .info-icon:hover .tooltip { display: block; }
  .info-icon:hover { z-index: 50; }
  .tooltip strong {
    display: block;
    font-size: 13px;
    color: var(--text-primary);
    margin-bottom: 6px;
  }
  .tooltip p {
    font-size: 11px;
    color: var(--text-secondary);
    line-height: 1.5;
    margin-bottom: 10px;
  }
  .tooltip-details {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 11px;
    color: var(--text-muted);
    border-top: 1px solid var(--border);
    padding-top: 8px;
  }
  .tooltip-details em {
    font-style: normal;
    color: var(--accent);
    font-weight: 600;
  }

  .action-details {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    margin-bottom: 12px;
    flex: 1;
  }
  .detail {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .detail-label { font-size: 10px; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.5px; }
  .detail-value { font-size: 13px; font-weight: 600; }
  .key-badge {
    background: rgba(124, 197, 209, 0.08);
    padding: 3px 10px;
    border: 1px solid rgba(124, 197, 209, 0.25);
    border-radius: 4px;
    font-family: 'Consolas', 'Courier New', monospace;
    color: var(--accent);
    font-size: 13px;
  }
  .cost-value { color: var(--accent); }
  .twitch-cd { color: #9146ff; }
  .lock-detail { width: 100%; }
  .limit-badge { color: var(--accent); font-size: 12px; font-weight: 600; white-space: nowrap; }
  .lock-badge { color: var(--warning); font-size: 12px; font-weight: 500; }

  .card-buttons { display: flex; gap: 6px; }
  .test-btn {
    width: auto;
    flex: 0 0 88px;
    border-color: rgba(124, 197, 209, 0.35);
    color: var(--accent);
  }
  .test-btn:disabled { opacity: 0.75; cursor: default; }

  .edit-btn {
    width: 100%;
    padding: 7px;
    background: transparent;
    border: 1px solid var(--border);
    color: var(--text-secondary);
    font-size: 11px;
    cursor: pointer;
    transition: all 0.15s;
    border-radius: var(--radius-sm);
    font-weight: 500;
  }
  .edit-btn:hover {
    border-color: rgba(124, 197, 209, 0.40);
    color: var(--accent);
    background: rgba(124, 197, 209, 0.08);
  }

  .toggle-switch { position: relative; display: inline-block; width: 38px; height: 20px; }
  .toggle-switch input { opacity: 0; width: 0; height: 0; }
  .toggle-slider {
    position: absolute;
    cursor: pointer;
    top: 0; left: 0; right: 0; bottom: 0;
    background: var(--border);
    border-radius: 20px;
    transition: 0.2s;
  }
  .toggle-slider:before {
    content: "";
    position: absolute;
    height: 14px; width: 14px;
    left: 3px; bottom: 3px;
    background: white;
    border-radius: 50%;
    transition: 0.2s;
  }
  .toggle-switch input:checked + .toggle-slider { background: var(--accent); }
  .toggle-switch input:checked + .toggle-slider:before { transform: translateX(18px); }

  .modal-overlay {
    position: fixed;
    top: 0; left: 0; right: 0; bottom: 0;
    background: rgba(0,0,0,0.75);
    backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }
  .modal {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 28px;
    width: 520px;
    max-height: 80vh;
    overflow-y: auto;
    box-shadow: 0 20px 60px rgba(0,0,0,0.5);
  }
  .modal-header { margin-bottom: 20px; }
  .modal-header h3 {
    color: var(--ink);
    font-size: 16px;
    font-weight: 700;
  }
  .modal-subtitle {
    font-size: 12px;
    color: var(--text-muted);
  }

  .form-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 14px;
    margin-bottom: 18px;
  }
  .form-grid label, .keylock-section label {
    display: flex;
    flex-direction: column;
    gap: 5px;
  }
  .form-grid label span, .keylock-section label span {
    font-size: 11px;
    color: var(--text-secondary);
    font-weight: 500;
  }
  .form-grid input, .form-grid select,
  .keylock-section input[type="text"],
  .keylock-section input[type="number"] {
    padding: 8px 12px;
    background: var(--bg-input);
    border: 1px solid var(--border);
    color: var(--text-primary);
    border-radius: var(--radius-sm);
    font-size: 13px;
    transition: border-color 0.15s;
  }
  .form-grid input:focus, .form-grid select:focus,
  .keylock-section input:focus {
    outline: none;
    border-color: rgba(124, 197, 209, 0.50);
    box-shadow: 0 0 0 2px rgba(124, 197, 209, 0.12);
  }
  .form-grid select {
    cursor: pointer;
  }

  .keylock-section {
    margin-bottom: 20px;
    padding: 14px;
    background: var(--bg-secondary);
    border-radius: var(--radius);
    border: 1px solid var(--border);
  }
  .checkbox-label {
    flex-direction: row !important;
    align-items: center;
    gap: 10px !important;
    margin-bottom: 10px;
    cursor: pointer;
  }

  .modal-buttons {
    display: flex;
    gap: 10px;
    justify-content: flex-end;
  }
  .btn-save {
    padding: 8px 24px;
    background: var(--accent);
    border: none;
    color: var(--ink);
    cursor: pointer;
    border-radius: var(--radius-sm);
    font-weight: 700;
    font-size: 13px;
    transition: all 0.15s;
  }
  .btn-save:hover { filter: brightness(1.1); }
  .btn-cancel {
    padding: 8px 24px;
    background: transparent;
    border: 1px solid var(--border);
    color: var(--text-secondary);
    cursor: pointer;
    border-radius: var(--radius-sm);
    font-size: 13px;
  }
  .btn-cancel:hover { border-color: var(--text-secondary); color: var(--text-primary); }

  .color-row {
    display: flex;
    gap: 8px;
    align-items: center;
  }
  .color-picker {
    width: 40px;
    height: 34px;
    padding: 2px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--bg-input);
    cursor: pointer;
  }
  .color-picker::-webkit-color-swatch-wrapper { padding: 2px; }
  .color-picker::-webkit-color-swatch { border-radius: 3px; border: none; }
  .color-text {
    flex: 1;
    font-family: 'Consolas', monospace;
    text-transform: uppercase;
  }

  .add-btn {
    width: 32px;
    height: 32px;
    background: rgba(124, 197, 209, 0.08);
    border: 1px solid rgba(124, 197, 209, 0.25);
    color: var(--accent);
    font-size: 20px;
    font-weight: 700;
    cursor: pointer;
    border-radius: var(--radius-sm);
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.15s;
  }
  .add-btn:hover {
    background: rgba(124, 197, 209, 0.15);
    border-color: rgba(124, 197, 209, 0.50);
  }

  .custom-badge {
    font-size: 9px;
    font-weight: 700;
    color: #9146ff;
    background: rgba(145, 71, 255, 0.12);
    border: 1px solid rgba(145, 71, 255, 0.3);
    padding: 1px 6px;
    border-radius: 3px;
    margin-left: 8px;
    letter-spacing: 0.5px;
    vertical-align: middle;
  }

  .btn-delete {
    padding: 8px 24px;
    background: var(--danger-dim);
    border: 1px solid rgba(224, 85, 85, 0.3);
    color: var(--danger);
    cursor: pointer;
    border-radius: var(--radius-sm);
    font-size: 13px;
    font-weight: 600;
    transition: all 0.15s;
  }
  .btn-delete:hover {
    background: var(--danger);
    color: white;
  }
</style>
