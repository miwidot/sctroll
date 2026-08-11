<script>
  import Header from './components/Header.svelte'
  import Sidebar from './components/Sidebar.svelte'
  import ActionsPanel from './components/ActionsPanel.svelte'
  import TwitchSetup from './components/TwitchSetup.svelte'
  import EventLog from './components/EventLog.svelte'
  import Settings from './components/Settings.svelte'

  let currentTab = 'actions'
</script>

<div class="app-container">
  <Header />
  <div class="main-layout">
    <Sidebar bind:currentTab />
    <main class="content">
      {#if currentTab === 'actions'}
        <ActionsPanel />
      {:else if currentTab === 'twitch'}
        <TwitchSetup />
      {:else if currentTab === 'settings'}
        <Settings />
      {/if}
      <div class="log-wrapper" class:hidden={currentTab !== 'log'}>
        <EventLog />
      </div>
    </main>
  </div>
</div>

<style>
  .app-container {
    height: 100vh;
    display: flex;
    flex-direction: column;
    background: var(--bg-primary);
  }

  .main-layout {
    flex: 1;
    display: flex;
    overflow: hidden;
  }

  .content {
    flex: 1;
    padding: 24px;
    overflow-y: auto;
  }

  .log-wrapper {
    display: contents;
  }
  .log-wrapper.hidden {
    display: none;
  }
</style>
