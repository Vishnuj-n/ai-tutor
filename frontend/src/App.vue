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
  -ms-overflow-style: none;
  scrollbar-width: none;
}

.content-shell::-webkit-scrollbar {
  width: 0;
  height: 0;
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
