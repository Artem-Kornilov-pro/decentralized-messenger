// Formats a Date as Go's time.RFC3339Nano would, for a UTC time truncated to
// millisecond precision (which is what models.NewMessage produces server-side
// — see internal/models/message.go). JS Date is inherently millisecond
// precision, so this always operates on a whole-millisecond value.
//
// RFC3339Nano keeps a variable-width fractional second: trailing zeros are
// stripped, and the fractional part (and its leading dot) is omitted
// entirely when the value is exactly on a second boundary.
export function formatTimestamp(date: Date): string {
  const pad = (n: number, width = 2) => String(n).padStart(width, '0')
  const y = pad(date.getUTCFullYear(), 4)
  const mo = pad(date.getUTCMonth() + 1)
  const d = pad(date.getUTCDate())
  const h = pad(date.getUTCHours())
  const mi = pad(date.getUTCMinutes())
  const s = pad(date.getUTCSeconds())
  const ms = date.getUTCMilliseconds()

  let frac = ''
  if (ms !== 0) {
    const nanos9 = String(ms * 1_000_000).padStart(9, '0')
    frac = '.' + nanos9.replace(/0+$/, '')
  }
  return `${y}-${mo}-${d}T${h}:${mi}:${s}${frac}Z`
}
