<template>
  <article :class="['banner', `banner--${variant}`, `${legacyPrefix}-banner`, 'card']">
    <div class="banner-content">
      <span class="banner-icon">{{ icon }}</span>
      <div class="banner-text">
        <p :class="['banner-title', `${legacyPrefix}-title`]">{{ title }}</p>
        <p v-if="subtitle" class="banner-subtitle">{{ subtitle }}</p>
      </div>
    </div>
  </article>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  /** 'info' | 'rescue' | 'success' | 'error' */
  variant: { type: String, required: true },
  icon: { type: String, required: true },
  title: { type: String, required: true },
  subtitle: { type: String, default: '' },
})

const LEGACY_PREFIXES = {
  info: 'info',
  rescue: 'rescue',
  success: 'flashcard-success',
  error: 'error',
}
const legacyPrefix = computed(() => LEGACY_PREFIXES[props.variant] || props.variant)
</script>

<style scoped>
.banner {
  background: var(--surface-container-lowest);
  border: 1px solid var(--outline-variant);
  border-radius: 16px;
}

.banner-content {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 12px;
}

.banner-icon {
  width: 40px;
  height: 40px;
  color: white;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 700;
  flex-shrink: 0;
}

.banner-title {
  margin: 0 0 4px;
  font-size: 15px;
  font-weight: 700;
}

.banner-subtitle {
  margin: 0;
  font-size: 13px;
}

/* --- Variant: info (escape hatch) --- */
.banner--info {
  background: rgba(230, 126, 34, 0.1);
  border: 1px solid rgba(230, 126, 34, 0.2);
}
.banner--info .banner-icon {
  background: #e67e22;
}
.banner--info .banner-title {
  color: #e67e22;
}

/* --- Variant: rescue (socratic rescue) --- */
.banner--rescue {
  background: rgba(211, 84, 0, 0.1);
  border: 1px solid rgba(211, 84, 0, 0.2);
}
.banner--rescue .banner-icon {
  background: #d35400;
}
.banner--rescue .banner-title {
  color: #d35400;
}

/* --- Variant: success (flashcard creation) --- */
.banner--success {
  background: rgba(46, 204, 113, 0.1);
  border-color: rgba(46, 204, 113, 0.2);
}
.banner--success .banner-icon {
  background: #2ecc71;
}
.banner--success .banner-title {
  color: #2ecc71;
}

/* --- Variant: error --- */
.banner--error {
  background: rgba(235, 94, 85, 0.1);
  border-color: rgba(235, 94, 85, 0.2);
}
.banner--error .banner-icon {
  background: #eb5e55;
}
.banner--error .banner-title {
  color: #eb5e55;
}
</style>
