<template>
  <article class="panel form-grid">
    <h2>Workspace Aesthetics</h2>
    <div class="form-group">
      <label for="theme-select">Aesthetic Theme</label>
      <select id="theme-select" v-model="settings.theme" :disabled="disabled">
        <option value="light-classic">Light Classic</option>
        <option value="light-warm">Warm Sepia (Reader)</option>
        <option value="dark-indigo">Deep Indigo Night (Dark Mode)</option>
        <option value="dark-nord">Nord Frost (Cool Dark Mode)</option>
        <option value="dark-emerald">Forest Emerald</option>
      </select>
      <p class="hint">
        Select a visual theme. Changing themes alters the colors of your study desk instantly.
      </p>
    </div>
  </article>
</template>

<script setup>
defineProps({
  settings: { type: Object, required: true },
  disabled: { type: Boolean, default: false },
})
</script>

<style scoped>
label {
  font-weight: 600;
  font-size: 14px;
  color: var(--on-surface);
}

select {
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  border-radius: 12px;
  background: var(--surface-container-low);
  color: var(--on-surface);
  padding: 12px 14px;
  font-size: 14px;
  font-family: inherit;
  transition:
    border-color 0.2s ease,
    box-shadow 0.2s ease;
}

select:focus {
  border-color: var(--primary);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--primary) 15%, transparent);
  outline: none;
}

.hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--muted-text);
  line-height: 1.4;
}

.form-grid {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

h2 {
  font-size: 20px;
  margin: 0 0 16px;
  font-weight: 700;
}

.panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 28px;
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  box-shadow: 0 4px 20px color-mix(in srgb, var(--on-surface) 3%, transparent);
}
</style>
