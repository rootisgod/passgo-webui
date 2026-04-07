import { defineStore } from 'pinia'
import { listVMs, listLaunches, listGroups, listProfiles, dismissLaunch, getHostResources, ApiError } from '../api/client.js'
import { recordMetrics } from '../composables/useMetricsHistory.js'

export const useVmStore = defineStore('vms', {
  state: () => ({
    authenticated: false,
    vms: [],
    launches: [],  // in-progress or recently-failed launches
    selectedNode: null,  // null = host, string = VM name
    selectedVms: [],     // multi-select: array of VM names
    lastRefresh: null,
    loading: false,
    error: null,
    hostname: 'localhost',
    hostResources: null,  // { total_cpus, load_avg_1, ..., total_memory_mb, used_memory_mb, total_disk_mb, used_disk_mb }
    // Groups
    groups: [],           // ordered list of group names
    vmGroups: {},         // {vmName: groupName}
    expandedGroups: {},   // {groupName: bool} local UI state
    // Profiles
    profiles: [],
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
    selectedVmObjects: (state) => state.vms.filter(vm => state.selectedVms.includes(vm.name)),
    ungroupedVms: (state) => state.vms.filter(vm => !state.vmGroups[vm.name]),
  },

  actions: {
    groupedVms(groupName) {
      return this.vms.filter(vm => this.vmGroups[vm.name] === groupName)
    },

    groupSummary(groupName) {
      const vms = this.groupedVms(groupName)
      return {
        running: vms.filter(v => v.state === 'Running').length,
        stopped: vms.filter(v => v.state === 'Stopped').length,
        total: vms.length,
      }
    },

    async fetchVMs() {
      try {
        this.loading = true
        this.error = null
        const [data, launchData, hostData] = await Promise.all([
          listVMs(),
          listLaunches().catch(() => []),
          getHostResources().catch(() => null),
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

        // Record metrics for running VMs
        for (const vm of vms) {
          if (vm.state === 'Running' && vm.load) {
            const loadParts = vm.load.split(' ').map(Number)
            const cpuLoad = loadParts.length >= 1 ? loadParts[0] : 0
            const memPct = vm.memory_total_raw ? (vm.memory_usage_raw / vm.memory_total_raw) * 100 : 0
            const diskPct = vm.disk_total_raw ? (vm.disk_usage_raw / vm.disk_total_raw) * 100 : 0
            recordMetrics(vm.name, { cpu: cpuLoad, memory: memPct, disk: diskPct })
          }
        }

        // Refresh groups and profiles alongside VMs
        await Promise.all([this.fetchGroups(), this.fetchProfiles()])

        // Record host resource metrics
        if (hostData) {
          this.hostResources = hostData
          const memPct = hostData.total_memory_mb ? (hostData.used_memory_mb / hostData.total_memory_mb) * 100 : 0
          const diskPct = hostData.total_disk_mb ? (hostData.used_disk_mb / hostData.total_disk_mb) * 100 : 0
          recordMetrics('__host__', { cpu: hostData.load_avg_1 || 0, memory: memPct, disk: diskPct })
        }
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

    async fetchGroups() {
      try {
        const data = await listGroups()
        this.groups = data.groups || []
        this.vmGroups = data.vmGroups || {}
      } catch {
        // Non-critical — keep existing state
      }
    },

    async fetchProfiles() {
      try {
        const data = await listProfiles()
        this.profiles = Array.isArray(data) ? data : []
      } catch {
        // Non-critical — keep existing state
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

    toggleVmSelection(name) {
      const idx = this.selectedVms.indexOf(name)
      if (idx >= 0) {
        this.selectedVms.splice(idx, 1)
      } else {
        this.selectedVms.push(name)
      }
    },

    selectAllVms() {
      this.selectedVms = this.vms.map(vm => vm.name)
    },

    clearSelection() {
      this.selectedVms = []
    },

    toggleGroupExpanded(name) {
      this.expandedGroups[name] = !this.expandedGroups[name]
    },
  },
})
