<script setup>
import { onMounted } from 'vue'
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
import Toast from './components/shared/Toast.vue'

const store = useVmStore()

// Try to resume session on load
onMounted(async () => {
  try {
    const ver = await getVersion()
    store.hostname = ver.hostname || 'localhost'
    // If version works, try listing VMs to check auth
    await store.fetchVMs()
    if (!store.error || store.error.indexOf('401') === -1) {
      store.authenticated = true
    }
  } catch {
    // Not authenticated
  }
})

// Polling (only when authenticated)
usePolling(() => {
  if (store.authenticated) {
    store.fetchVMs()
  }
}, 3000)
</script>

<template>
  <Toast />

  <LoginPage v-if="!store.authenticated" />

  <div v-else class="h-screen flex flex-col">
    <AppHeader />
    <div class="flex flex-1 overflow-hidden">
      <TreeSidebar />
      <main class="flex-1 overflow-auto">
        <CloudInitPanel v-if="store.selectedNode === '__cloud-init__'" />
        <Transition v-else name="fade" mode="out-in">
          <HostPanel v-if="store.selectedNode === null" key="host" />
          <VmDetailPanel v-else :key="store.selectedNode" />
        </Transition>
      </main>
    </div>
    <StatusBar />
  </div>
</template>
