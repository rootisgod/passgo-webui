import { defineStore } from 'pinia'
import { listVMs, listLaunches, dismissLaunch, ApiError } from '../api/client.js'

export const useVmStore = defineStore('vms', {
  state: () => ({
    authenticated: false,
    vms: [],
    launches: [],  // in-progress or recently-failed launches
    selectedNode: null,  // null = host, string = VM name
    lastRefresh: null,
    loading: false,
    error: null,
    hostname: 'localhost',
  }),

  getters: {
    selectedVm: (state) => state.vms.find(vm => vm.name === state.selectedNode),
    runningCount: (state) => state.vms.filter(vm => vm.state === 'Running').length,
    stoppedCount: (state) => state.vms.filter(vm => vm.state === 'Stopped').length,
    suspendedCount: (state) => state.vms.filter(vm => vm.state === 'Suspended').length,
    deletedCount: (state) => state.vms.filter(vm => vm.state === 'Deleted').length,
    totalCount: (state) => state.vms.length,
    // Only show launches for VMs not yet in the real VM list
    activeLaunches: (state) => {
      const vmNames = new Set(state.vms.map(vm => vm.name))
      return state.launches.filter(l => l.status === 'launching' && !vmNames.has(l.name))
    },
    launchingCount() { return this.activeLaunches.length },
    failedLaunches: (state) => state.launches.filter(l => l.status === 'failed'),
  },

  actions: {
    async fetchVMs() {
      try {
        this.loading = true
        this.error = null
        const [data, launchData] = await Promise.all([
          listVMs(),
          listLaunches().catch(() => []),
        ])
        const launches = Array.isArray(launchData) ? launchData : []
        const launchingNames = new Set(launches.filter(l => l.status === 'launching').map(l => l.name))
        const vms = Array.isArray(data) ? data : []
        // Tag VMs that are still being created with a "Creating" state
        for (const vm of vms) {
          if (launchingNames.has(vm.name) && (!vm.state || vm.state === 'Unknown')) {
            vm.state = 'Creating'
          }
        }
        this.vms = vms
        this.launches = launches
        this.lastRefresh = new Date()
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          this.authenticated = false
          return
        }
        this.error = err.message
      } finally {
        this.loading = false
      }
    },

    async dismissFailedLaunch(name) {
      try {
        await dismissLaunch(name)
        this.launches = this.launches.filter(l => l.name !== name)
      } catch { /* ignore */ }
    },

    selectNode(name) {
      this.selectedNode = name
    },
  },
})
