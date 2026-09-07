export interface Comp {
  external_id: string
  title: string
  price: string
  currency: string
  condition?: string
  buying_option?: string
  item_url?: string
  thumbnail_url?: string
  seller_country?: string
  excluded: boolean
}

export interface Search {
  id: string
  /** The signed-in user who uploaded this search. Results are shared, so
   * this is the only thing distinguishing whose find a card is. */
  user_email: string
  /** Server's answer to "may this viewer change it?" — never re-derived
   * client side, so the UI and the API can't disagree. */
  is_owner: boolean
  status: string
  error_message?: string
  image_url: string
  title?: string
  brand?: string
  model?: string
  category?: string
  condition_notes?: string
  search_query?: string
  confidence?: string
  low_confidence?: boolean
  price_source?: string
  currency?: string
  comp_count?: number
  price_mean?: string
  price_median?: string
  price_min?: string
  price_max?: string
  price_trimmed_mean?: string
  comps?: Comp[]
  created_at: string
  completed_at?: string
  /** Client-only: set when polling hit MAX_POLL_ATTEMPTS without a terminal status. */
  still_working?: boolean
}

interface ListSearchesResponse {
  items: Search[]
  next_cursor?: string
}

interface ListSearchersResponse {
  /** Emails that have run at least one search. */
  items: string[]
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
  const searchers = ref<string[]>([])
  /** The search the detail page is showing. It's held separately from
   * `items` because a shared search is readable even when the active
   * filter excludes it from the grid. */
  const current = ref<Search | null>(null)
  /** null means "everyone" — the shared, unfiltered view. */
  const emailFilter = ref<string | null>(null)

  // Polling state lives outside the reactive store state — it's bookkeeping,
  // not UI state, and keeping it here (rather than in the component) means
  // navigating away from the page doesn't orphan a poller.
  const pollTimers = new Map<string, ReturnType<typeof setTimeout>>()
  const pollAttempts = new Map<string, number>()

  // Bumped by every first-page request; responses from an older generation
  // (a different filter) are discarded instead of applied.
  let pageGeneration = 0

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

  function stopAllPolls() {
    for (const id of [...pollTimers.keys()]) {
      stopPoll(id)
    }
  }

  /** listUrl builds the list request for the active filter. */
  function listUrl(cursor?: string) {
    const params = new URLSearchParams({ limit: '24' })
    if (emailFilter.value) {
      params.set('email', emailFilter.value)
    }
    if (cursor) {
      params.set('cursor', cursor)
    }
    return `/api/searches?${params.toString()}`
  }

  /** normalizeFilter mirrors the server's normalization (see
   * normalizeEmail in httpapi) so a hand-typed or shared `?email=Alice@X`
   * still matches the rows the API returns. */
  function normalizeFilter(email: string | null) {
    const normalized = email?.trim().toLowerCase() ?? ''
    return normalized === '' ? null : normalized
  }

  function matchesFilter(search: Search) {
    return emailFilter.value === null || search.user_email === emailFilter.value
  }

  /** upsertItem updates a row in place wherever it came from, but only
   * inserts one the active filter would show — otherwise polling a
   * detail page or a late poll response could drop a foreign card into a
   * filtered grid. */
  function upsertItem(detail: Search) {
    if (current.value?.id === detail.id) {
      current.value = { ...current.value, ...detail }
    }

    const idx = items.value.findIndex(i => i.id === detail.id)
    if (idx === -1) {
      if (matchesFilter(detail)) {
        items.value.unshift(detail)
      }
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

  /** fetchDetail does a single, unbounded fetch of a search's full detail
   * (including comps) and throws on failure — unlike poll(), which
   * swallows errors to keep retrying, the detail page needs to
   * distinguish "doesn't exist / not yours" from a transient blip. */
  async function fetchDetail(id: string): Promise<Search> {
    const detail = await apiFetch<Search>(`/api/searches/${id}`)
    current.value = detail
    upsertItem(detail)
    return detail
  }

  /** rerun re-prices a search with an edited query, then resumes polling. */
  async function rerun(id: string, searchQuery: string) {
    await apiFetch(`/api/searches/${id}/rerun`, {
      method: 'POST',
      body: { search_query: searchQuery }
    })
    poll(id)
  }

  /** loadSearchers refreshes the filter control's options. A failure here
   * only costs the filter chips, so it never surfaces as an error. */
  async function loadSearchers() {
    try {
      const res = await apiFetch<ListSearchersResponse>('/api/searches/searchers')
      searchers.value = res.items
    } catch {
      // Leave the previous options in place.
    }
  }

  /** fetchPage replaces the grid with the first page for the active
   * filter. Each call takes a generation number: a response that arrives
   * after a newer request started (two quick chip clicks) is dropped
   * rather than painting one searcher's rows under another's chip. */
  async function fetchPage() {
    const generation = ++pageGeneration
    loading.value = true
    // The page is being replaced wholesale (a filter switch, a re-login),
    // so pollers for rows that are about to disappear are wasted requests.
    stopAllPolls()
    try {
      const res = await apiFetch<ListSearchesResponse>(listUrl())
      if (generation !== pageGeneration) {
        return
      }

      items.value = res.items
      nextCursor.value = res.next_cursor ?? null
      initialized.value = true

      for (const item of res.items) {
        if (!isTerminal(item.status)) {
          poll(item.id)
        }
      }
    } finally {
      if (generation === pageGeneration) {
        loading.value = false
      }
    }
  }

  /** loadInitial loads the home page: the first page of searches for
   * `email` (null for everyone's), plus the filter control's options,
   * concurrently. */
  async function loadInitial(email: string | null = null) {
    emailFilter.value = normalizeFilter(email)
    void loadSearchers()
    await fetchPage()
  }

  /** setEmailFilter narrows the grid to one searcher, or shows everyone's
   * searches when passed null. The option list can't change here, so it
   * isn't refetched. */
  async function setEmailFilter(email: string | null) {
    const normalized = normalizeFilter(email)
    if (emailFilter.value === normalized) {
      return
    }
    emailFilter.value = normalized
    await fetchPage()
  }

  async function loadMore() {
    if (!nextCursor.value || loadingMore.value) {
      return
    }
    loadingMore.value = true
    const generation = pageGeneration
    try {
      const res = await apiFetch<ListSearchesResponse>(listUrl(nextCursor.value))
      if (generation !== pageGeneration) {
        // The filter changed while this page was in flight; its rows and
        // its cursor belong to a list that's no longer on screen.
        return
      }

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
    const auth = useAuthStore()
    const email = auth.user?.email ?? ''

    // Uploading while looking at someone else's searches would leave the
    // new card invisible, so return to the shared view first. If that
    // refresh fails the grid is merely stale — the upload still goes
    // ahead, and reporting it as an upload error would be a lie.
    if (emailFilter.value !== email) {
      await setEmailFilter(null).catch(() => {})
    }

    const tempId = `pending-${Date.now()}-${Math.random().toString(36).slice(2)}`
    const previewUrl = URL.createObjectURL(file)

    items.value.unshift({
      id: tempId,
      user_email: email,
      is_owner: true,
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
      if (email && !searchers.value.includes(email)) {
        searchers.value = [...searchers.value, email].sort()
      }
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
    if (current.value?.id === id) {
      current.value = null
    }
  }

  return {
    items,
    nextCursor,
    loading,
    loadingMore,
    initialized,
    searchers,
    emailFilter,
    current,
    loadInitial,
    loadMore,
    setEmailFilter,
    create,
    poll,
    retry,
    remove,
    fetchDetail,
    rerun
  }
})
