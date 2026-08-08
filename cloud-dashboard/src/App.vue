<template>
  <div class="dashboard-container ds-root" :class="{ light: isLight }">
    <Navbar :is-light="isLight" @toggle-theme="toggleTheme" />

    <main class="main-content">
      <!-- Toast Notification -->
      <div
        v-if="toastMessage"
        class="animate-fade-in"
        style="background: var(--ds-success-bg); border: 1px solid var(--ds-success-ring); color: var(--ds-success); padding: 0.75rem 1rem; border-radius: 8px; font-size: 0.85rem; display: flex; align-items: center; gap: 0.5rem; margin-bottom: 1rem;"
      >
        <div style="flex: 1;">{{ toastMessage }}</div>
      </div>

      <!-- Error Bar -->
      <div v-if="error" class="error-message" style="margin-bottom: 1rem;">
        <div style="flex: 1;">{{ error }}</div>
      </div>

      <RouterView />
    </main>
  </div>
</template>

<script setup>
import { ref, provide, onMounted, onUnmounted } from 'vue';
import Navbar from './components/Navbar.vue';
import { useDashboard } from './composables/useDashboard';

const isLight = ref(localStorage.getItem('cloud_dashboard_theme') === 'light');

function toggleTheme() {
  isLight.value = !isLight.value;
  localStorage.setItem('cloud_dashboard_theme', isLight.value ? 'light' : 'dark');
}

provide('theme', { isLight, toggleTheme });

const {
  toastMessage,
  error,
  searchInputRef,
  initSession,
  stopPolling
} = useDashboard();

const handleGlobalKeydown = (e) => {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault();
    searchInputRef.value?.focus();
  } else if (e.key === '/' && document.activeElement !== searchInputRef.value && !['INPUT', 'TEXTAREA'].includes(document.activeElement?.tagName)) {
    e.preventDefault();
    searchInputRef.value?.focus();
  }
};

onMounted(() => {
  initSession();
  window.addEventListener('keydown', handleGlobalKeydown);
});

onUnmounted(() => {
  window.removeEventListener('keydown', handleGlobalKeydown);
  stopPolling();
});
</script>

<style>
/* Global dashboard styles loaded from style.css */
</style>

