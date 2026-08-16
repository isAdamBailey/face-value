/**
 * useMoney formats prices with Intl.NumberFormat using the comp's own
 * currency — never a hardcoded "$", since a future marketplace could be
 * priced in anything.
 */
export function useMoney() {
  function format(amount: string | number | undefined, currency: string | undefined): string {
    if (amount === undefined || currency === undefined) {
      return '—'
    }
    const value = typeof amount === 'string' ? Number(amount) : amount
    if (Number.isNaN(value)) {
      return '—'
    }
    return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(value)
  }

  return { format }
}
