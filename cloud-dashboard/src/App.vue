<template>
  <div class="dashboard-container">
    <Navbar />

    <main class="main-content">
      <!-- Toast Notification -->
      <div
        v-if="toastMessage"
        class="animate-fade-in"
        style="background: rgba(16, 185, 129, 0.1); border: 1px solid rgba(16, 185, 129, 0.3); color: var(--success); padding: 0.75rem 1rem; border-radius: 8px; font-size: 0.85rem; display: flex; align-items: center; gap: 0.5rem; margin-bottom: 1rem;"
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
import { onMounted, onUnmounted } from 'vue';
import Navbar from './components/Navbar.vue';
import { useDashboard } from './composables/useDashboard';

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
