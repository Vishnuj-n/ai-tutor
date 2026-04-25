/**
 * Format FSRS rating value for display
 * @param {string} raw - Raw rating value from API
 * @returns {string} Formatted rating display text
 */
export function formatRating(raw) {
  const value = String(raw || '').toLowerCase()
  if (value === 'again') return 'Again'
  if (value === 'hard') return 'Hard'
  if (value === 'good') return 'Good'
  if (value === 'easy') return 'Easy'
  return 'Unrated'
}

/**
 * Format next review timestamp for display
 * @param {string} raw - Raw timestamp string from API
 * @returns {string} Formatted date/time display text
 */
export function formatNextReview(raw) {
  const value = String(raw || '').trim()
  if (!value) return 'Not scheduled'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString([], {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}
