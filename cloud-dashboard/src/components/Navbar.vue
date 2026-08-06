<template>
  <header class="header">
    <div style="display: flex; align-items: center; gap: 1.5rem;">
      <div>
        <h1>
          <span>AI Tutor</span>
          <span style="color: var(--primary); font-weight: 500;"> Portal</span>
        </h1>
        <div class="subtitle">Teacher Analytical Workspace</div>
      </div>

      <nav v-if="!showSetup" class="nav-tabs" style="display: flex; gap: 0.5rem; margin-left: 1rem;">
        <RouterLink to="/overview" class="nav-tab">Overview</RouterLink>
        <RouterLink to="/students" class="nav-tab">Students</RouterLink>
        <RouterLink to="/assignments" class="nav-tab">Assignments</RouterLink>
      </nav>
    </div>

    <div style="display: flex; align-items: center; gap: 1.25rem;">
      <div
        v-if="!showSetup"
        style="display: flex; align-items: center; gap: 0.5rem; background: rgba(16, 185, 129, 0.04); border: 1px solid rgba(16, 185, 129, 0.15); padding: 0.3rem 0.6rem; border-radius: 6px;"
        :title="'Last auto-polled: ' + syncTimeAgo"
      >
        <span class="pulsing-dot" style="background-color: var(--success); box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.4);"></span>
        <span style="font-size: 0.7rem; color: var(--success); font-weight: 600; font-family: var(--font-mono); letter-spacing: 0.02em;">
          LIVE SYNCED • {{ syncTimeAgo }}
        </span>
      </div>

      <span v-if="classroomCode" class="classroom-badge">
        CLASSROOM: {{ classroomCode }}
      </span>

      <button v-if="!showSetup" class="btn btn-secondary" @click="onLogout" style="padding: 0.45rem 0.85rem; font-size: 0.8rem;">
        Sign Out
      </button>
    </div>
  </header>
</template>

<script setup>
import { useRouter } from 'vue-router';
import { useDashboard } from '../composables/useDashboard';

const router = useRouter();
const { showSetup, syncTimeAgo, classroomCode, logoutTeacher } = useDashboard();

function onLogout() {
  logoutTeacher(router);
}
</script>

<style scoped>
.nav-tab {
  padding: 0.45rem 0.9rem;
  border-radius: 6px;
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--muted-text);
  text-decoration: none;
  transition: all 0.2s ease;
  border: 1px solid transparent;
}

.nav-tab:hover {
  color: var(--on-surface);
  background: var(--surface-low);
}

.nav-tab.router-link-active {
  color: var(--primary);
  background: var(--surface-low);
  border-color: color-mix(in srgb, var(--primary) 30%, transparent);
}
</style>
