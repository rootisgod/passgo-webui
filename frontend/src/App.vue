<script setup>
import { onMounted } from 'vue'
import { useVmStore } from './stores/vmStore.js'
import { usePolling } from './composables/usePolling.js'
import { getVersion } from './api/client.js'

import AppHeader from './components/layout/AppHeader.vue'
import TreeSidebar from './components/layout/TreeSidebar.vue'
import StatusBar from './components/layout/StatusBar.vue'
import HostPanel from './components/host/HostPanel.vue'
import VmDetailPanel from './components/vm/VmDetailPanel.vue'
import CloudInitPanel from './components/cloudinit/CloudInitPanel.vue'
import Toast from './components/shared/Toast.vue'

const store = useVmStore()

onMounted(async () => {
  try {
    const ver = await getVersion()
    store.hostname = ver.hostname || 'localhost'
    await store.fetchVMs()
  } catch {
    // Server not reachable
  }
})

usePolling(() => {
  store.fetchVMs()
}, 3000)
</script>

<template>
  <Toast />

  <div class="h-screen flex flex-col">
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
