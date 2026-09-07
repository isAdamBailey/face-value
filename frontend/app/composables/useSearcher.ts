/**
 * useSearcher names whoever ran a search. Results are shared across every
 * login, so a card has to say whose find it is — your own reads as "you",
 * everyone else by the local part of their address, which is short enough
 * for a tile and enough to tell people apart. The full address goes on a
 * title attribute at the call site.
 *
 * Ownership itself is not derived here: the API ships `is_owner` on every
 * search, so the UI and the server can't disagree about who may change one.
 */
export function useSearcher() {
  const auth = useAuthStore()

  function label(email: string | undefined): string {
    if (!email) {
      return 'someone'
    }
    if (email === auth.user?.email) {
      return 'you'
    }
    return email.split('@')[0] || email
  }

  return { label }
}
