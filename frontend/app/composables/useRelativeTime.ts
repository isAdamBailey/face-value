const DIVISIONS: Array<[Intl.RelativeTimeFormatUnit, number]> = [
  ['year', 60 * 60 * 24 * 365],
  ['month', 60 * 60 * 24 * 30],
  ['week', 60 * 60 * 24 * 7],
  ['day', 60 * 60 * 24],
  ['hour', 60 * 60],
  ['minute', 60]
]

/** useRelativeTime formats an ISO timestamp as "3 minutes ago" etc. */
export function useRelativeTime() {
  const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })

  function format(iso: string): string {
    const diffSeconds = (new Date(iso).getTime() - Date.now()) / 1000

    for (const [unit, secondsInUnit] of DIVISIONS) {
      if (Math.abs(diffSeconds) >= secondsInUnit) {
        return rtf.format(Math.round(diffSeconds / secondsInUnit), unit)
      }
    }
    return rtf.format(Math.round(diffSeconds), 'second')
  }

  return { format }
}
