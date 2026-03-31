import { defineStore } from 'pinia'

let nextId = 0

export const useToastStore = defineStore('toast', {
  state: () => ({
    toasts: [],
  }),

  actions: {
    add(message, type = 'success') {
      const id = nextId++
      this.toasts.push({ id, message, type })
      setTimeout(() => this.remove(id), 4000)
    },

    success(message) { this.add(message, 'success') },
    error(message) { this.add(message, 'error') },
    info(message) { this.add(message, 'info') },

    remove(id) {
      const idx = this.toasts.findIndex(t => t.id === id)
      if (idx !== -1) this.toasts.splice(idx, 1)
    },
  },
})
