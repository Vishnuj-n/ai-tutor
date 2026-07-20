/**
 * UI utility helpers extracted from Dashboard.vue.
 * Pure functions — no Vue reactivity, no API calls.
 */

export const MONTH_NAMES = [
  'January',
  'February',
  'March',
  'April',
  'May',
  'June',
  'July',
  'August',
  'September',
  'October',
  'November',
  'December',
]

/**
 * Format a Date object to a local "YYYY-MM-DD" string.
 * @param {Date} date
 * @returns {string}
 */
export function getLocalDateString(date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

/**
 * Human-readable label for days remaining until a deadline.
 * @param {number} days
 * @returns {string}
 */
export function formatDaysRemaining(days) {
  if (days === 0) return 'today!'
  if (days < 0) return 'passed'
  return `${days} days left`
}

/**
 * Map a task action_type to a human-readable label.
 * @param {string} type
 * @returns {string}
 */
export function formatTaskType(type) {
  const t = (type || '').toLowerCase()
  if (t === 'reading') return 'Reading'
  if (t === 'flashcard_review') return 'Review'
  if (t === 'quiz') return 'Quiz'
  if (t === 'examiner') return 'Examiner'
  if (t === 'reread') return 'Reread'
  if (t === 'socratic_remedial') return 'Concept Rescue'
  if (t === 'flashcard_generate') return 'Generate Flashcards'
  if (t === 'milestone_exam') return 'Milestone Exam'
  return type
}

/**
 * Build the calendar grid array for a given month.
 * Returns padding cells (dayNum: null) followed by day cells with
 * { dayNum, dateString, active, today } shape.
 *
 * @param {number} year       — Full year, e.g. 2026
 * @param {number} month      — Zero-based month index (0 = January)
 * @param {string[]} activeDates — Array of "YYYY-MM-DD" strings with activity
 * @returns {Array<{dayNum: number|null, dateString: string|null, active: boolean, today: boolean}>}
 */
export function buildCalendarDays(year, month, activeDates = []) {
  const days = []
  const totalDays = new Date(year, month + 1, 0).getDate()
  const firstDay = new Date(year, month, 1).getDay()

  for (let i = 0; i < firstDay; i++) {
    days.push({ dayNum: null, dateString: null, active: false, today: false })
  }

  const todayLocalStr = getLocalDateString(new Date())

  for (let d = 1; d <= totalDays; d++) {
    const dStr = `${year}-${String(month + 1).padStart(2, '0')}-${String(d).padStart(2, '0')}`
    const isActive = activeDates.includes(dStr)
    const isToday = dStr === todayLocalStr

    days.push({
      dayNum: d,
      dateString: dStr,
      active: isActive,
      today: isToday,
    })
  }

  return days
}
