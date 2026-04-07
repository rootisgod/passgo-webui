<script setup>
import { ref, onMounted } from 'vue'
import { useVmStore } from './stores/vmStore.js'
import { usePolling } from './composables/usePolling.js'
import { getVersion } from './api/client.js'

import LoginPage from './components/LoginPage.vue'
import AppHeader from './components/layout/AppHeader.vue'
import TreeSidebar from './components/layout/TreeSidebar.vue'
import StatusBar from './components/layout/StatusBar.vue'
import HostPanel from './components/host/HostPanel.vue'
import VmDetailPanel from './components/vm/VmDetailPanel.vue'
import CloudInitPanel from './components/cloudinit/CloudInitPanel.vue'
import SettingsPanel from './components/settings/SettingsPanel.vue'
import ProfilesPanel from './components/profiles/ProfilesPanel.vue'
import AnsiblePanel from './components/ansible/AnsiblePanel.vue'
import SchedulesPanel from './components/schedule/SchedulesPanel.vue'
import Toast from './components/shared/Toast.vue'
import ChatPanel from './components/chat/ChatPanel.vue'

const store = useVmStore()
const checkingAuth = ref(true)

onMounted(async () => {
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
