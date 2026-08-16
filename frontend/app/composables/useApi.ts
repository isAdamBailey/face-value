const CSRF_COOKIE_NAME = 'facevalue_csrf'

interface ApiFetchOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'
  body?: unknown
}

/**
 * apiFetch calls the backend API, sending session cookies and, for
 * state-changing requests, the CSRF token from the facevalue_csrf cookie as
 * the X-CSRF-Token header (double-submit pattern).
 */
export function apiFetch<T>(path: string, options: ApiFetchOptions = {}): Promise<T> {
  const config = useRuntimeConfig()
  const method = options.method ?? 'GET'

  const headers: Record<string, string> = {}
  const isFormData = typeof FormData !== 'undefined' && options.body instanceof FormData
  if (options.body !== undefined && !isFormData) {
    headers['Content-Type'] = 'application/json'
  }
  if (method !== 'GET') {
    const csrfToken = useCookie(CSRF_COOKIE_NAME).value
    if (csrfToken) {
      headers['X-CSRF-Token'] = csrfToken
    }
  }

  return $fetch<T>(path, {
    baseURL: config.public.apiBase,
    method,
    credentials: 'include',
    headers,
    body: options.body as BodyInit | Record<string, unknown> | null | undefined
  })
}
