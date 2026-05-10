<template>
  <section class="study-page">
    <header class="study-head">
      <div class="study-head__label">
        <p class="eyebrow">{{ eyebrow }}</p>
        <h1 class="page-title">{{ title }}</h1>
        <p v-if="subtitle" class="page-subtitle">{{ subtitle }}</p>
      </div>
      <div v-if="$slots.toolbar" class="study-head__toolbar">
        <slot name="toolbar" />
      </div>
    </header>

    <slot />
  </section>
</template>

<script setup>
defineProps({
  eyebrow:  { type: String, default: '' },
  title:    { type: String, required: true },
  subtitle: { type: String, default: '' },
})
</script>

<style scoped>
.study-page {
  display: grid;
  gap: 16px;
}

.study-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.study-head__label {
  display: grid;
  gap: 4px;
}

.eyebrow {
  margin: 0;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: var(--muted-text);
}

.page-title {
  margin: 0;
  font-family: 'Manrope', sans-serif;
  font-size: 40px;
  font-weight: 700;
  letter-spacing: -0.03em;
  line-height: 1;
  color: var(--on-surface);
}

.page-subtitle {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
  letter-spacing: 0.01em;
}

.study-head__toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  flex-wrap: wrap;
}

@media (max-width: 900px) {
  .study-head {
    flex-direction: column;
    align-items: flex-start;
  }

  .page-title {
    font-size: 32px;
  }
}
</style>
