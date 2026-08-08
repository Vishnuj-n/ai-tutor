<template>
  <header class="header">
    <div style="display: flex; align-items: center; gap: 1.5rem;">
      <div>
        <h1 style="display: flex; align-items: center; gap: 0.4rem; font-size: 0.95rem;">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--ds-primary)" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <polygon points="12 2 2 7 12 12 22 7 12 2"></polygon>
            <polyline points="2 17 12 22 22 17"></polyline>
            <polyline points="2 12 12 17 22 12"></polyline>
          </svg>
          <span>StudyLoop</span>
          <span style="color: var(--ds-primary); font-weight: 500;"> Portal</span>
        </h1>
        <div class="subtitle">Teacher Analytical Workspace</div>
      </div>

      <nav v-if="!showSetup" class="nav-tabs" style="display: flex; gap: 0.25rem; margin-left: 0.75rem;">
        <RouterLink to="/overview" class="nav-tab">Overview</RouterLink>
        <RouterLink to="/students" class="nav-tab">Students</RouterLink>
        <RouterLink to="/assignments" class="nav-tab">Assignments</RouterLink>
      </nav>
    </div>

    <div style="display: flex; align-items: center; gap: 0.75rem;">
      <div v-if="!showSetup" class="badge-live" :title="'Last auto-polled: ' + syncTimeAgo">
        <span class="pulse-dot"></span>
        <span>LIVE SYNCED • {{ syncTimeAgo }}</span>
      </div>

      <span v-if="classroomCode" class="badge-class">
        CLASSROOM: {{ classroomCode }}
      </span>

      <!-- Theme Toggle Button -->
      <button
        type="button"
        class="btn-ghost"
        style="padding: 0.35rem 0.5rem; width: 32px; height: 32px; justify-content: center;"
        :title="isLight ? 'Switch to dark mode' : 'Switch to light mode'"
        @click="emit('toggle-theme')"
      >
        <!-- Sun icon for light, Moon for dark -->
        <svg v-if="isLight" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path>
        </svg>
        <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="5"></circle>
          <line x1="12" y1="1" x2="12" y2="3"></line>
          <line x1="12" y1="21" x2="12" y2="23"></line>
          <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"></line>
          <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"></line>
          <line x1="1" y1="12" x2="3" y2="12"></line>
          <line x1="21" y1="12" x2="23" y2="12"></line>
          <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"></line>
          <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"></line>
        </svg>
      </button>

      <button v-if="!showSetup" class="btn-ghost" @click="onLogout" title="Sign out of portal">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path>
          <polyline points="16 17 21 12 16 7"></polyline>
          <line x1="21" y1="12" x2="9" y2="12"></line>
        </svg>
        <span>Sign Out</span>
      </button>
    </div>
  </header>
</template>

<script setup>
import { useRouter } from 'vue-router';
import { useDashboard } from '../composables/useDashboard';

defineProps({
  isLight: {
    type: Boolean,
    default: false
  }
});

const emit = defineEmits(['toggle-theme']);

const router = useRouter();
const { showSetup, syncTimeAgo, classroomCode, logoutTeacher } = useDashboard();

function onLogout() {
  logoutTeacher(router);
}
</script>

<style scoped>
.nav-tab {
  padding: 0.35rem 0.8rem;
  border-radius: 6px;
  font-size: 0.8rem;
  font-weight: 500;
  color: var(--ds-muted);
  text-decoration: none;
  transition: all 0.2s ease;
  border: 1px solid transparent;
}

.nav-tab:hover {
  color: var(--ds-fg);
  background: var(--ds-surface-hi);
}

.nav-tab.router-link-active {
  color: var(--ds-primary);
  background: var(--ds-primary-glow);
  border-color: var(--ds-primary-ring);
}
</style>

