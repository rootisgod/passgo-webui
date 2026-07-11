import { describe, expect, it } from 'vitest'
import { createSerializedRunner } from './usePolling.js'

function deferred() {
  let resolve
  const promise = new Promise((done) => { resolve = done })
  return { promise, resolve }
}

describe('createSerializedRunner', () => {
  it('coalesces overlapping requests into one follow-up run', async () => {
    const pending = []
    let callCount = 0
    const run = createSerializedRunner(() => {
      callCount += 1
      const next = deferred()
      pending.push(next)
      return next.promise
    })

    const first = run()
    run()
    run()
    await Promise.resolve()
    expect(callCount).toBe(1)

    pending[0].resolve()
    await first
    await Promise.resolve()
    expect(callCount).toBe(2)

    pending[1].resolve()
    await Promise.resolve()
    expect(callCount).toBe(2)
  })

  it('does not run while inactive', async () => {
    let called = false
    const run = createSerializedRunner(() => { called = true }, () => false)
    await run()
    expect(called).toBe(false)
  })
})
