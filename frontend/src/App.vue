<script setup>
import Sidebar from './components/Sidebar.vue'
</script>

<template>
  <div class="app-shell">
    <Sidebar />

    <main class="content-shell">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  min-height: 100vh;
  display: flex;
  background: var(--background);
}

.content-shell {
  flex: 1;
  padding: 24px;
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
