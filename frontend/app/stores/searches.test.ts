import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useSearchesStore, type Search } from './searches'

function makeSearch(overrides: Partial<Search> = {}): Search {
  return {
    id: 'id-1',
    user_email: 'owner@example.com',
    is_owner: true,
    status: 'pending',
    image_url: 'https://example.com/img.jpg',
    created_at: new Date().toISOString(),
    ...overrides
  }
}

/** stubFetch routes $fetch by path so a test only spells out the responses
 * it cares about: the searcher list, the list page, and detail polls. */
function stubFetch(routes: {
  searchers?: string[]
  list?: (path: string) => unknown
  detail?: () => unknown
}) {
  const mock = vi.fn(async (path: string) => {
    if (path.startsWith('/api/searches/searchers')) {
      return { items: routes.searchers ?? [] }
    }
    if (path.startsWith('/api/searches?')) {
      return routes.list?.(path) ?? { items: [] }
    }
    return routes.detail?.() ?? makeSearch()
  })
  vi.stubGlobal('$fetch', mock)
  return mock
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

    stubFetch({
      list: path => path.includes('cursor=')
        ? { items: moreItems, next_cursor: undefined }
        : { items: initialItems, next_cursor: 'cursor-1' }
    })

    const store = useSearchesStore()
    await store.loadInitial()
    expect(store.items.map(i => i.id)).toEqual(['a', 'b'])

    await store.loadMore()
    expect(store.items.map(i => i.id)).toEqual(['a', 'b', 'c'])
  })

  it('scopes list requests to the filtered address, including pagination', async () => {
    const fetchMock = stubFetch({
      searchers: ['owner@example.com', 'other@example.com'],
      list: () => ({ items: [], next_cursor: 'cursor-1' })
    })

    const store = useSearchesStore()
    await store.loadInitial()
    await flushMicrotasks()

    const paths = () => fetchMock.mock.calls.map(c => c[0])
    expect(paths()).toContain('/api/searches?limit=24')
    expect(store.searchers).toEqual(['owner@example.com', 'other@example.com'])

    await store.setEmailFilter('owner@example.com')
    expect(paths()).toContain('/api/searches?limit=24&email=owner%40example.com')

    // The filter must survive pagination, or scrolling would silently widen
    // the list back to everyone.
    await store.loadMore()
    expect(paths()).toContain('/api/searches?limit=24&email=owner%40example.com&cursor=cursor-1')

    // Switching filters is not a page load, so the options aren't refetched.
    expect(paths().filter(p => p === '/api/searches/searchers')).toHaveLength(1)
  })

  it('loadInitial applies the filter it is given', async () => {
    const fetchMock = stubFetch({})

    const store = useSearchesStore()
    await store.loadInitial('owner@example.com')

    expect(store.emailFilter).toBe('owner@example.com')
    expect(fetchMock.mock.calls.map(c => c[0]))
      .toContain('/api/searches?limit=24&email=owner%40example.com')
  })

  it('a filtered-out search is still readable, without entering the grid', async () => {
    stubFetch({
      list: () => ({ items: [], next_cursor: undefined }),
      detail: () => makeSearch({ id: 'a', user_email: 'other@example.com', status: 'complete' })
    })

    const store = useSearchesStore()
    await store.loadInitial('owner@example.com')
    await flushMicrotasks()

    // fetchDetail is how the detail page loads any shared search: it must
    // not drop a foreign card into a filtered grid, and the detail page
    // must still get something to render.
    await store.fetchDetail('a')
    expect(store.items).toEqual([])
    expect(store.current?.id).toBe('a')
  })

  it('normalizes the filter the way the server does', async () => {
    const fetchMock = stubFetch({})

    const store = useSearchesStore()
    await store.loadInitial('  Owner@Example.com  ')

    expect(store.emailFilter).toBe('owner@example.com')
    expect(fetchMock.mock.calls.map(c => c[0]))
      .toContain('/api/searches?limit=24&email=owner%40example.com')

    // Same address, differently cased: no second request.
    const before = fetchMock.mock.calls.length
    await store.setEmailFilter('OWNER@example.com')
    expect(fetchMock.mock.calls).toHaveLength(before)
  })

  it('drops a list response that a newer filter has superseded', async () => {
    const resolvers: Array<(value: unknown) => void> = []
    const fetchMock = vi.fn((path: string) => {
      if (path.startsWith('/api/searches/searchers')) {
        return Promise.resolve({ items: [] })
      }
      return new Promise((resolve) => {
        resolvers.push(resolve)
      })
    })
    vi.stubGlobal('$fetch', fetchMock)

    const store = useSearchesStore()
    void store.setEmailFilter('slow@example.com')
    const settled = store.setEmailFilter('fast@example.com')

    // Resolve the newer request first, then let the stale one land.
    resolvers[1]!({ items: [makeSearch({ id: 'fast', user_email: 'fast@example.com' })] })
    await settled
    resolvers[0]!({ items: [makeSearch({ id: 'slow', user_email: 'slow@example.com' })] })
    await flushMicrotasks()

    expect(store.emailFilter).toBe('fast@example.com')
    expect(store.items.map(i => i.id)).toEqual(['fast'])
  })

  it('stops pollers for rows the new page replaced', async () => {
    const fetchMock = stubFetch({
      list: path => path.includes('email=')
        ? { items: [], next_cursor: undefined }
        : { items: [makeSearch({ id: 'a', status: 'pricing' })], next_cursor: undefined },
      detail: () => makeSearch({ id: 'a', status: 'pricing' })
    })

    const store = useSearchesStore()
    await store.loadInitial()
    await flushMicrotasks()

    const detailCalls = () => fetchMock.mock.calls.filter(c => c[0] === '/api/searches/a').length
    expect(detailCalls()).toBe(1)

    await store.setEmailFilter('someone-else@example.com')
    await vi.advanceTimersByTimeAsync(20000)

    expect(detailCalls()).toBe(1)
    expect(store.items).toEqual([])
  })
})
