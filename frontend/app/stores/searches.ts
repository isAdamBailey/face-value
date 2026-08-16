export interface Search {
  id: string
  status: string
  error_message?: string
  image_url: string
  title?: string
  brand?: string
  model?: string
  comp_count?: number
  currency?: string
  price_trimmed_mean?: string
  created_at: string
  /** Client-only: set when polling hit MAX_POLL_ATTEMPTS without a terminal status. */
  still_working?: boolean
}

interface ListSearchesResponse {
  items: Search[]
  next_cursor?: string
}

interface CreateSearchResponse {
  id: string
  status: string
}

const TERMINAL_STATUSES = new Set(['complete', 'failed'])
const POLL_INTERVAL_MS = 2000
const MAX_POLL_ATTEMPTS = 60

export const useSearchesStore = defineStore('searches', () => {
  const items = ref<Search[]>([])
  const nextCursor = ref<string | null>(null)
  const loading = ref(false)
  const loadingMore = ref(false)
  const initialized = ref(false)

  // Polling state lives outside the reactive store state — it's bookkeeping,
  // not UI state, and keeping it here (rather than in the component) means
  // navigating away from the page doesn't orphan a poller.
  const pollTimers = new Map<string, ReturnType<typeof setTimeout>>()
  const pollAttempts = new Map<string, number>()

  function isTerminal(status: string) {
    return TERMINAL_STATUSES.has(status)
  }

  function stopPoll(id: string) {
    const timer = pollTimers.get(id)
    if (timer !== undefined) {
      clearTimeout(timer)
      pollTimers.delete(id)
    }
    pollAttempts.delete(id)
  }

  function upsertItem(detail: Search) {
    const idx = items.value.findIndex(i => i.id === detail.id)
    if (idx === -1) {
      items.value.unshift(detail)
      return
    }

    const existing = items.value[idx]!
    if (existing.image_url?.startsWith('blob:') && detail.image_url && detail.image_url !== existing.image_url) {
      URL.revokeObjectURL(existing.image_url)
    }
    items.value[idx] = { ...existing, ...detail }
  }

  /** poll begins bounded polling of a search's status every 2s, up to
   * MAX_POLL_ATTEMPTS, stopping as soon as the status is terminal. */
  function poll(id: string) {
    stopPoll(id)
    pollAttempts.set(id, 0)
    void tick()

    async function tick() {
      const attempt = (pollAttempts.get(id) ?? 0) + 1
      pollAttempts.set(id, attempt)

      try {
        const detail = await apiFetch<Search>(`/api/searches/${id}`)
        upsertItem(detail)

        if (isTerminal(detail.status)) {
          stopPoll(id)
          return
        }
      } catch {
        // Transient fetch error — keep polling until attempts run out
        // rather than giving up on the first blip.
      }

      if (attempt >= MAX_POLL_ATTEMPTS) {
        stopPoll(id)
        const item = items.value.find(i => i.id === id)
        if (item) {
          item.still_working = true
        }
        return
      }

      pollTimers.set(id, setTimeout(tick, POLL_INTERVAL_MS))
    }
  }

  /** retry restarts bounded polling — for a card stuck in "still working"
   * after the poll cap, or to re-check a failed row. */
  function retry(id: string) {
    const item = items.value.find(i => i.id === id)
    if (item) {
      item.still_working = false
    }
    poll(id)
  }

  async function loadInitial() {
    loading.value = true
    try {
      const res = await apiFetch<ListSearchesResponse>('/api/searches?limit=24')
      items.value = res.items
      nextCursor.value = res.next_cursor ?? null
      initialized.value = true

      for (const item of res.items) {
        if (!isTerminal(item.status)) {
          poll(item.id)
        }
      }
    } finally {
      loading.value = false
    }
  }

  async function loadMore() {
    if (!nextCursor.value || loadingMore.value) {
      return
    }
    loadingMore.value = true
    try {
      const res = await apiFetch<ListSearchesResponse>(
        `/api/searches?limit=24&cursor=${encodeURIComponent(nextCursor.value)}`
      )
      const existingIds = new Set(items.value.map(i => i.id))
      const fresh = res.items.filter(i => !existingIds.has(i.id))
      items.value.push(...fresh)
      nextCursor.value = res.next_cursor ?? null
    } finally {
      loadingMore.value = false
    }
  }

  /** create uploads file, optimistically prepends a skeleton card using a
   * local object-URL preview, and begins polling once the server assigns
   * a real id. */
  async function create(file: File) {
    const tempId = `pending-${Date.now()}-${Math.random().toString(36).slice(2)}`
    const previewUrl = URL.createObjectURL(file)

    items.value.unshift({
      id: tempId,
      status: 'pending',
      image_url: previewUrl,
      created_at: new Date().toISOString()
    })

    try {
      const formData = new FormData()
      formData.append('image', file)

      const res = await apiFetch<CreateSearchResponse>('/api/searches', {
        method: 'POST',
        body: formData
      })

      const idx = items.value.findIndex(i => i.id === tempId)
      if (idx !== -1) {
        items.value[idx] = { ...items.value[idx]!, id: res.id, status: res.status }
      }

      poll(res.id)
    } catch (err) {
      URL.revokeObjectURL(previewUrl)
      items.value = items.value.filter(i => i.id !== tempId)
      throw err
    }
  }

  async function remove(id: string) {
    await apiFetch(`/api/searches/${id}`, { method: 'DELETE' })
    stopPoll(id)

    const item = items.value.find(i => i.id === id)
    if (item?.image_url?.startsWith('blob:')) {
      URL.revokeObjectURL(item.image_url)
    }
    items.value = items.value.filter(i => i.id !== id)
  }

  return {
    items,
    nextCursor,
    loading,
    loadingMore,
    initialized,
    loadInitial,
    loadMore,
    create,
    poll,
    retry,
    remove
  }
})
