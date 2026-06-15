<script setup>
import ProxyRulesPanel from '../proxy/ProxyRulesPanel.vue'

defineProps({
  vmName: { type: String, required: true },
})
</script>

<template>
  <ProxyRulesPanel :vm-name="vmName" embedded />
</template>
