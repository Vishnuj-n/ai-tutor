<script setup>
import Sidebar from './components/Sidebar.vue'
import { useRoute } from 'vue-router'
import { onMounted } from 'vue'
import { getUserSettings } from './services/appApi'

const route = useRoute()

onMounted(async () => {
  try {
    const res = await getUserSettings()
    if (res && res.theme) {
      document.documentElement.setAttribute('data-theme', res.theme)
    }
  } catch (err) {
    console.error('Failed to load global theme:', err)
  }
})
</script>

<template>
  <div class="app-shell">
    <Sidebar v-if="route.path !== '/onboarding'" />

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
