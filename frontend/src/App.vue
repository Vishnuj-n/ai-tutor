<script setup>
import Sidebar from './components/Sidebar.vue'
import { useRoute } from 'vue-router'
import { onMounted, onUnmounted, ref } from 'vue'
import { getUserSettings, updateUserSettings, getTodayPlan } from './services/appApi'

const route = useRoute()

const banner = ref({
  show: false,
  type: 'start',
  title: '',
  desc: '',
  unfinishedCount: 0,
})

let schedulerTimeout = null

// Helper to parse "HH:MM" into a Date object on a given base date
function parseTime(timeStr, baseDate) {
  const [h, m] = timeStr.split(':').map(Number)
  const d = new Date(baseDate)
  d.setHours(h, m, 0, 0)
  return d
}

// Calculate delay in ms and event type for next closest start or end time
function getNextEventTimeout(startTimeStr, endTimeStr) {
  if (!startTimeStr || !endTimeStr) return null
  const now = new Date()
  const events = []

  // Start time event
  const startToday = parseTime(startTimeStr, now)
  if (startToday > now) {
    events.push({ type: 'start', time: startToday })
  } else {
    const startTomorrow = parseTime(startTimeStr, new Date(now.getTime() + 86400000))
    events.push({ type: 'start', time: startTomorrow })
  }

  // End time event
  const endToday = parseTime(endTimeStr, now)
  if (endToday > now) {
    events.push({ type: 'end', time: endToday })
  } else {
    const endTomorrow = parseTime(endTimeStr, new Date(now.getTime() + 86400000))
    events.push({ type: 'end', time: endTomorrow })
  }

  // Sort events to find the closest upcoming one
  events.sort((a, b) => a.time - b.time)
  const next = events[0]
  return {
    type: next.type,
    delay: next.time.getTime() - now.getTime(),
  }
}

// Fire notifications and banners based on event type
async function fireEvent(type) {
  if (type === 'start') {
    if ('Notification' in window && Notification.permission === 'granted') {
      new Notification('Study Time Started!', {
        body: "It's study time! Let's work on today's learning queue.",
      })
    }
    banner.value = {
      show: true,
      type: 'start',
      title: 'Study Time Started!',
      desc: 'Your study window has started. Time to work on your queue!',
      unfinishedCount: 0,
    }
  } else if (type === 'end') {
    let unfinishedCount = 0
    try {
      const plan = await getTodayPlan()
      if (plan && plan.tasks) {
        unfinishedCount = plan.tasks.length
      }
    } catch (err) {
      console.error('Failed to check tasks at end time:', err)
    }

    if (unfinishedCount > 0) {
      if ('Notification' in window && Notification.permission === 'granted') {
        new Notification('Study Time is Up!', {
          body: `You still have ${unfinishedCount} unfinished tasks today.`,
        })
      }
      banner.value = {
        show: true,
        type: 'end',
        title: 'Study Time is Up!',
        desc: `You still have ${unfinishedCount} unfinished study tasks remaining today.`,
        unfinishedCount,
      }
    } else {
      if ('Notification' in window && Notification.permission === 'granted') {
        new Notification('Study Time is Up!', {
          body: 'Great job! You finished all your study tasks for today.',
        })
      }
      banner.value = {
        show: true,
        type: 'end',
        title: 'Study Time is Up!',
        desc: 'Great job! You finished all your study tasks for today.',
        unfinishedCount: 0,
      }
    }
  }
}

// Fetch settings and schedule next setTimeout
async function syncScheduler() {
  if (schedulerTimeout) {
    clearTimeout(schedulerTimeout)
    schedulerTimeout = null
  }

  try {
    const settings = await getUserSettings()
    if (!settings || settings.error) return

    // Apply theme
    if (settings.theme) {
      document.documentElement.setAttribute('data-theme', settings.theme)
    }

    if (!settings.reminders_enabled) return

    // Request notification permission if reminders are enabled
    if ('Notification' in window && Notification.permission === 'default') {
      Notification.requestPermission()
    }

    const next = getNextEventTimeout(settings.study_start_time, settings.study_end_time)
    if (!next) return

    // Schedule next timeout
    schedulerTimeout = setTimeout(async () => {
      await fireEvent(next.type)
      syncScheduler() // Queue up the next event
    }, next.delay)
  } catch (err) {
    console.error('Scheduler sync failed:', err)
  }
}

// Extend study window by X minutes
async function extendStudyWindow(minutes) {
  try {
    const settings = await getUserSettings()
    if (!settings || settings.error) return

    const [h, m] = settings.study_end_time.split(':').map(Number)
    let newMins = h * 60 + m + minutes
    if (newMins >= 1440) {
      newMins -= 1440 // Midnight wrap-around
    }

    const newH = Math.floor(newMins / 60)
      .toString()
      .padStart(2, '0')
    const newM = (newMins % 60).toString().padStart(2, '0')
    const newEndTimeStr = `${newH}:${newM}`

    const res = await updateUserSettings(
      settings.max_flashcards_per_session,
      settings.study_start_time,
      newEndTimeStr,
      settings.reminders_enabled,
      settings.active_profile_id,
      settings.skip_to_reading_active,
      settings.cloud_sync_url,
      settings.cloud_api_token,
      settings.theme,
      settings.rag_enabled,
      settings.rag_notebook_chapter,
      settings.rag_entire_notebook,
      settings.rag_queue_study,
      settings.default_remedial_strategy
    )

    if (res.error) {
      console.error('Failed to extend study window:', res.error)
      return
    }

    window.dispatchEvent(new CustomEvent('settings-updated'))
    banner.value.show = false
  } catch (err) {
    console.error('Extend study window failed:', err)
  }
}

function closeBanner() {
  banner.value.show = false
}

onMounted(() => {
  syncScheduler()
  window.addEventListener('settings-updated', syncScheduler)
})

onUnmounted(() => {
  if (schedulerTimeout) {
    clearTimeout(schedulerTimeout)
    schedulerTimeout = null
  }
  window.removeEventListener('settings-updated', syncScheduler)
})
</script>

<template>
  <div class="app-shell">
    <Sidebar v-if="route.path !== '/onboarding'" />

    <main class="content-shell">
      <!-- Global study reminder banner -->
      <div v-if="banner.show" class="study-alert-banner">
        <div class="banner-content">
          <span class="banner-icon">{{ banner.type === 'start' ? '⏰' : '⏳' }}</span>
          <div class="banner-text">
            <strong class="banner-title">{{ banner.title }}</strong>
            <p class="banner-desc">{{ banner.desc }}</p>
          </div>
        </div>
        <div class="banner-actions">
          <template v-if="banner.type === 'end' && banner.unfinishedCount > 0">
            <button class="banner-btn primary" @click="extendStudyWindow(15)">+15 mins</button>
            <button class="banner-btn primary" @click="extendStudyWindow(30)">+30 mins</button>
          </template>
          <button class="banner-btn secondary" @click="closeBanner">Dismiss</button>
        </div>
      </div>

      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  width: 100%;
  height: 100vh;
  display: flex;
  background: var(--background);
  overflow: hidden;
}

.content-shell {
  flex: 1;
  min-height: 0;
  padding: 16px 20px;
  overflow-y: auto;
  scrollbar-width: none;
  position: relative;
}

.content-shell::-webkit-scrollbar {
  width: 0;
  height: 0;
}

/* ── Global study reminder banner ── */
.study-alert-banner {
  position: fixed;
  top: 16px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 9999;
  background: var(--card-bg, rgba(30, 41, 59, 0.95));
  border: 1px solid var(--accent, #4f46e5);
  box-shadow:
    0 10px 25px -5px rgba(0, 0, 0, 0.3),
    0 8px 10px -6px rgba(0, 0, 0, 0.3);
  backdrop-filter: blur(12px);
  border-radius: 12px;
  padding: 14px 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  width: 90%;
  max-width: 600px;
  animation: slideDown 0.4s cubic-bezier(0.16, 1, 0.3, 1) forwards;
}

@keyframes slideDown {
  from {
    transform: translate(-50%, -30px);
    opacity: 0;
  }
  to {
    transform: translate(-50%, 0);
    opacity: 1;
  }
}

.banner-content {
  display: flex;
  align-items: center;
  gap: 12px;
}

.banner-icon {
  font-size: 1.5rem;
}

.banner-text {
  display: flex;
  flex-direction: column;
}

.banner-title {
  color: var(--text, #ffffff);
  font-size: 0.95rem;
  font-weight: 600;
}

.banner-desc {
  color: var(--text-muted, #94a3b8);
  font-size: 0.85rem;
  margin: 2px 0 0 0;
}

.banner-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.banner-btn {
  border: none;
  border-radius: 6px;
  padding: 6px 12px;
  font-size: 0.8rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.banner-btn.primary {
  background: var(--accent, #4f46e5);
  color: white;
}

.banner-btn.primary:hover {
  background: var(--accent-hover, #4338ca);
}

.banner-btn.secondary {
  background: transparent;
  color: var(--text-muted, #94a3b8);
  border: 1px solid var(--border, rgba(255, 255, 255, 0.1));
}

.banner-btn.secondary:hover {
  background: rgba(255, 255, 255, 0.05);
  color: var(--text, #ffffff);
}

@media (max-width: 960px) {
  .app-shell {
    flex-direction: column;
  }

  .content-shell {
    padding: 16px;
  }
}
</style>
