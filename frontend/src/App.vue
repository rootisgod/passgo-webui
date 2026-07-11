<script setup>
import { defineAsyncComponent, ref, onMounted, onUnmounted } from 'vue'
import { useVmStore } from './stores/vmStore.js'
import { useToastStore } from './stores/toastStore.js'
import { usePolling } from './composables/usePolling.js'
import { getVersion } from './api/client.js'

import LoginPage from './components/LoginPage.vue'
import AppHeader from './components/layout/AppHeader.vue'
import TreeSidebar from './components/layout/TreeSidebar.vue'
import StatusBar from './components/layout/StatusBar.vue'
import Toast from './components/shared/Toast.vue'

const HostPanel = defineAsyncComponent(() => import('./components/host/HostPanel.vue'))
const VmDetailPanel = defineAsyncComponent(() => import('./components/vm/VmDetailPanel.vue'))
const CloudInitPanel = defineAsyncComponent(() => import('./components/cloudinit/CloudInitPanel.vue'))
const SettingsPanel = defineAsyncComponent(() => import('./components/settings/SettingsPanel.vue'))
const ProfilesPanel = defineAsyncComponent(() => import('./components/profiles/ProfilesPanel.vue'))
const AnsiblePanel = defineAsyncComponent(() => import('./components/ansible/AnsiblePanel.vue'))
const SchedulesPanel = defineAsyncComponent(() => import('./components/schedule/SchedulesPanel.vue'))
const ApiTokensPanel = defineAsyncComponent(() => import('./components/tokens/ApiTokensPanel.vue'))
const WebhooksPanel = defineAsyncComponent(() => import('./components/webhooks/WebhooksPanel.vue'))
const EventLogPanel = defineAsyncComponent(() => import('./components/events/EventLogPanel.vue'))
const ChatPanel = defineAsyncComponent(() => import('./components/chat/ChatPanel.vue'))

const store = useVmStore()
const toast = useToastStore()
const checkingAuth = ref(true)

// Any API 401 bubbles up here so callers don't each re-implement the logout flow.
function handleUnauthorized() {
  if (store.authenticated) {
    toast.error('Session expired — please log in again')
  }
  store.authenticated = false
}

onMounted(async () => {
  window.addEventListener('passgo:unauthorized', handleUnauthorized)
  try {
    const ver = await getVersion()
    store.hostname = ver.hostname || 'localhost'
    await store.fetchVMs()
    // fetchVMs catches 401 internally and sets authenticated=false,
    // so only mark authenticated if the fetch actually succeeded
    if (store.lastRefresh) {
      store.authenticated = true
    }
  } catch (e) {
    store.authenticated = false
  } finally {
    checkingAuth.value = false
  }
})

onUnmounted(() => {
  window.removeEventListener('passgo:unauthorized', handleUnauthorized)
})

usePolling(() => {
  if (store.authenticated) {
    store.fetchVMs()
  }
}, 3000)
</script>

<template>
  <!-- Show nothing while checking auth -->
  <div v-if="checkingAuth" />

  <LoginPage v-else-if="!store.authenticated" />

  <template v-else>
    <Toast />
    <div class="h-screen flex flex-col">
      <AppHeader />
      <div class="flex flex-1 overflow-hidden">
        <TreeSidebar />
        <main class="flex-1 overflow-auto">
          <CloudInitPanel v-if="store.selectedNode === '__cloud-init__'" />
          <AnsiblePanel v-else-if="store.selectedNode === '__ansible__'" />
          <ProfilesPanel v-else-if="store.selectedNode === '__profiles__'" />
          <SchedulesPanel v-else-if="store.selectedNode === '__schedules__'" />
          <WebhooksPanel v-else-if="store.selectedNode === '__webhooks__'" />
          <ApiTokensPanel v-else-if="store.selectedNode === '__api-tokens__'" />
          <EventLogPanel v-else-if="store.selectedNode === '__events__'" />
          <SettingsPanel v-else-if="store.selectedNode === '__settings__'" />
          <Transition v-else name="fade" mode="out-in">
            <HostPanel v-if="store.selectedNode === null" key="host" />
            <VmDetailPanel v-else :key="store.selectedNode" />
          </Transition>
        </main>
        <ChatPanel />
      </div>
      <StatusBar />
    </div>
  </template>
</template>
