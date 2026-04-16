<template>
  <WrittenAssessment v-if="route.name === 'written-assessment'" />

  <section v-else class="page">
    <p class="eyebrow">Tools</p>
    <h1>{{ pageTitle }}</h1>
    <article class="panel">
      <p>This tool surface is ready for implementation. Navigation is now wired.</p>
    </article>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import WrittenAssessment from './WrittenAssessment.vue'

const route = useRoute()

const pageTitle = computed(() => {
  const title = route.meta?.title
  return typeof title === 'string' && title.trim() !== '' ? title : 'Tool'
})
</script>

<style scoped>
.page {
  display: grid;
  gap: 20px;
}

.eyebrow {
  margin: 0;
  font-size: 12px;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  color: var(--muted-text);
  font-weight: 700;
}

h1 {
  margin: 0;
  font-size: 46px;
  font-family: 'Manrope', sans-serif;
  letter-spacing: -0.02em;
}

.panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 24px;
}

p {
  margin: 0;
  color: var(--muted-text);
  font-size: 15px;
}
</style>
