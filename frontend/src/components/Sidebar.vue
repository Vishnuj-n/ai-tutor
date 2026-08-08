<template>
  <aside class="sidebar">
    <div>
      <div class="brand">
        <div class="brand-mark">S</div>
        <div>
          <p class="brand-title">The StudyLoop</p>
        </div>
      </div>

      <nav class="menu">
        <RouterLink v-for="item in topItems" :key="item.to" :to="item.to" class="menu-item">
          <span class="menu-icon" aria-hidden="true">{{ item.icon }}</span>
          {{ item.label }}
        </RouterLink>
      </nav>
    </div>

    <div class="bottom-actions">
      <button
        v-if="isCloudAccount"
        class="sync-link"
        :class="{ 'sync-success': syncState === 'success', 'sync-error': syncState === 'error' }"
        type="button"
        @click="handleSync"
        :disabled="syncing"
        title="Sync with Cloud"
      >
        <span class="menu-icon">{{ syncIcon }}</span>
        <span>{{ syncLabel }}</span>
      </button>
      <RouterLink to="/settings" class="menu-item bottom-item">Settings</RouterLink>
    </div>
  </aside>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { triggerCloudSync, getUserSettings } from '../services/appApi'

const syncing = ref(false)
const syncState = ref('idle')
const isCloudAccount = ref(false)
let syncTimer = null

const syncIcon = computed(() => {
  if (syncState.value === 'success') return '✓'
  if (syncState.value === 'error') return '⚠️'
  return '☁️'
})

const syncLabel = computed(() => {
  if (syncState.value === 'syncing') return 'Syncing...'
  if (syncState.value === 'success') return 'Sync successful'
  if (syncState.value === 'error') return 'Sync failed'
  return 'Sync Cloud'
})

async function checkCloudAccount() {
  const s = await getUserSettings().catch(() => null)
  isCloudAccount.value = !!(s?.cloud_api_token || s?.classroom_code)
}

async function handleSync() {
  if (syncing.value) return
  if (syncTimer) {
    clearTimeout(syncTimer)
    syncTimer = null
  }
  syncing.value = true
  syncState.value = 'syncing'
  try {
    const res = await triggerCloudSync()
    if (res?.error) {
      console.warn('[SIDEBAR] Cloud sync error:', res.error)
      syncState.value = 'error'
    } else {
      console.log('[SIDEBAR] Cloud sync completed successfully')
      syncState.value = 'success'
    }
  } catch (err) {
    console.warn('[SIDEBAR] Cloud sync exception:', err)
    syncState.value = 'error'
  } finally {
    syncing.value = false
    syncTimer = setTimeout(() => {
      syncState.value = 'idle'
      syncTimer = null
    }, 4000)
  }
}

onMounted(() => {
  checkCloudAccount()
  window.addEventListener('settings-updated', checkCloudAccount)
})

onUnmounted(() => {
  window.removeEventListener('settings-updated', checkCloudAccount)
  if (syncTimer) {
    clearTimeout(syncTimer)
    syncTimer = null
  }
})

const topItems = [
  { to: '/dashboard', label: 'Dashboard', icon: '▦' },
  { to: '/reader', label: 'Reader', icon: '◫' },
  { to: '/notebooks', label: 'Notebooks', icon: '▤' },
  { to: '/quiz', label: 'Quiz', icon: '◪' },
  { to: '/flashcards', label: 'Flashcards', icon: '◧' },
  { to: '/examiner', label: 'Examiner', icon: '✎' },
  { to: '/tutor', label: 'Tutor', icon: '◎' },
]
</script>

<style scoped>
.sidebar {
  width: 248px;
  min-width: 248px;
  background: var(--surface-container);
  padding: 24px 16px 24px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 24px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 4px 8px;
}

.brand-mark {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  background: linear-gradient(15deg, var(--primary-dim), var(--primary));
  color: var(--on-primary);
  display: grid;
  place-items: center;
  font-family: 'Manrope', sans-serif;
  font-weight: 700;
}

.brand-title {
  margin: 0;
  font-family: 'Manrope', sans-serif;
  font-size: 20px;
  letter-spacing: -0.02em;
  font-weight: 700;
  color: var(--on-surface);
}

.brand-subtitle {
  margin: 2px 0 0;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.16em;
  color: var(--muted-text);
}

.menu {
  margin-top: 28px;
  display: grid;
  gap: 6px;
}

.menu-item {
  display: flex;
  align-items: center;
  gap: 9px;
  border-radius: 12px;
  text-decoration: none;
  color: var(--on-surface);
  padding: 11px 12px;
  font-size: 15px;
  font-weight: 500;
  background: transparent;
  transition:
    background-color 0.2s cubic-bezier(0.16, 1, 0.3, 1),
    color 0.2s cubic-bezier(0.16, 1, 0.3, 1),
    transform 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}

.menu-icon {
  width: 18px;
  min-width: 18px;
  text-align: center;
  color: color-mix(in srgb, var(--muted-text) 65%, var(--on-surface));
  font-size: 12px;
  line-height: 1;
}

.menu-item:hover {
  background: var(--surface-container-low);
  transform: translateX(4px);
}

.menu-item.router-link-active {
  background: var(--surface-container-lowest);
  color: var(--primary);
}

.menu-item.router-link-active .menu-icon {
  color: var(--primary);
}

.bottom-actions {
  display: grid;
  gap: 10px;
}

.focus-chip {
  border: 0;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.72);
  backdrop-filter: blur(20px);
  color: var(--on-surface);
  font-size: 14px;
  font-weight: 600;
  padding: 10px 14px;
}

.sync-link {
  display: flex;
  align-items: center;
  gap: 9px;
  border: 1px solid var(--outline-variant);
  border-radius: 12px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  font-size: 14px;
  font-weight: 500;
  padding: 10px 12px;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  width: 100%;
}

.sync-link:hover:not(:disabled) {
  background: var(--surface-container-highest);
  color: var(--primary);
  border-color: var(--primary);
}

.sync-link.sync-success {
  color: #10b981;
  border-color: rgba(16, 185, 129, 0.4);
  background: color-mix(in srgb, #10b981 12%, var(--surface-container-low));
}

.sync-link.sync-success .menu-icon {
  color: #10b981;
}

.sync-link.sync-error {
  color: #ef4444;
  border-color: rgba(239, 68, 68, 0.4);
  background: color-mix(in srgb, #ef4444 12%, var(--surface-container-low));
}

.sync-link.sync-error .menu-icon {
  color: #ef4444;
}

.sync-link:disabled {
  opacity: 0.6;
  cursor: wait;
}

.bottom-item {
  color: var(--muted-text);
}

@media (max-width: 960px) {
  .sidebar {
    width: 100%;
    min-width: 0;
    border-radius: 0;
    padding: 16px;
    gap: 16px;
  }

  .menu {
    margin-top: 16px;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .bottom-actions {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    align-items: center;
  }

  .sync-link,
  .bottom-item {
    text-align: center;
  }
}
</style>
