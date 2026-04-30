<script setup>
import { onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import Sidebar from './components/Sidebar.vue'
import { getStudentSettings } from './services/appApi'

const router = useRouter()
const route = useRoute()

onMounted(async () => {
  // Skip setup check if already on setup page
  if (route.path === '/setup') {
    return
  }

  try {
    const response = await getStudentSettings()
    if (response.error) {
      console.error('Failed to check student settings:', response.error)
      return
    }

    // Redirect to setup if student_id is not set
    if (!response.student_id) {
      router.push('/setup')
    }
  } catch (err) {
    console.error('Error checking student settings:', err)
  }
})
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
