import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useSearchesStore, type Search } from './searches'

function makeSearch(overrides: Partial<Search> = {}): Search {
  return {
    id: 'id-1',
    status: 'pending',
    image_url: 'https://example.com/img.jpg',
    created_at: new Date().toISOString(),
    ...overrides
  }
}

async function flushMicrotasks() {
  for (let i = 0; i < 5; i++) {
    await Promise.resolve()
  }
}

describe('useSearchesStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('stops polling once status becomes complete', async () => {
    let call = 0
    const fetchMock = vi.fn(async () => {
      call++
      return call === 1
        ? makeSearch({ status: 'pricing' })
        : makeSearch({ status: 'complete', price_trimmed_mean: '42', currency: 'USD' })
    })
    vi.stubGlobal('$fetch', fetchMock)

    const store = useSearchesStore()
    store.poll('id-1')

    await flushMicrotasks()
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(store.items.find(i => i.id === 'id-1')?.status).toBe('pricing')

    await vi.advanceTimersByTimeAsync(2000)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(store.items.find(i => i.id === 'id-1')?.status).toBe('complete')

    // Polling must have stopped: advancing further triggers no more calls.
    await vi.advanceTimersByTimeAsync(20000)
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('stops polling once status becomes failed', async () => {
    let call = 0
    const fetchMock = vi.fn(async () => {
      call++
      return call === 1
        ? makeSearch({ status: 'identifying' })
        : makeSearch({ status: 'failed', error_message: 'identify: boom' })
    })
    vi.stubGlobal('$fetch', fetchMock)

    const store = useSearchesStore()
    store.poll('id-1')
    await flushMicrotasks()
    expect(fetchMock).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(2000)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(store.items.find(i => i.id === 'id-1')?.status).toBe('failed')

    await vi.advanceTimersByTimeAsync(20000)
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('stops polling and flags still_working after the attempt cap', async () => {
    const fetchMock = vi.fn(async () => makeSearch({ status: 'pricing' }))
    vi.stubGlobal('$fetch', fetchMock)

    const store = useSearchesStore()
    store.poll('id-1')
    await flushMicrotasks()

    // 59 more 2s ticks after the immediate first call reaches the 60-attempt cap.
    for (let i = 0; i < 59; i++) {
      await vi.advanceTimersByTimeAsync(2000)
    }

    expect(fetchMock).toHaveBeenCalledTimes(60)
    expect(store.items.find(i => i.id === 'id-1')?.still_working).toBe(true)

    await vi.advanceTimersByTimeAsync(20000)
    expect(fetchMock).toHaveBeenCalledTimes(60)
  })

  it('loadMore does not duplicate rows that overlap with the initial page', async () => {
    const initialItems = [
      makeSearch({ id: 'a', status: 'complete' }),
      makeSearch({ id: 'b', status: 'complete' })
    ]
    // 'b' overlaps with the initial page — the API contract guarantees the
    // keyset cursor never does this, but the store must not trust that
    // blindly.
    const moreItems = [
      makeSearch({ id: 'b', status: 'complete' }),
      makeSearch({ id: 'c', status: 'complete' })
    ]

    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ items: initialItems, next_cursor: 'cursor-1' })
      .mockResolvedValueOnce({ items: moreItems, next_cursor: undefined })
    vi.stubGlobal('$fetch', fetchMock)

    const store = useSearchesStore()
    await store.loadInitial()
    expect(store.items.map(i => i.id)).toEqual(['a', 'b'])

    await store.loadMore()
    expect(store.items.map(i => i.id)).toEqual(['a', 'b', 'c'])
  })
})
