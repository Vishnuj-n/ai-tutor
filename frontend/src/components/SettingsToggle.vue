<template>
  <div class="form-group check-group">
    <label class="checkbox-container">
      <input
        :checked="modelValue"
        type="checkbox"
        :disabled="disabled"
        @change="$emit('update:modelValue', $event.target.checked)"
      />
      <span class="checkmark"></span>
      <div class="check-label">
        <strong>{{ title }}</strong>
        <p v-if="hint" class="hint">{{ hint }}</p>
      </div>
    </label>
  </div>
</template>

<script setup>
defineProps({
  modelValue: { type: Boolean, required: true },
  title: { type: String, required: true },
  hint: { type: String, default: '' },
  disabled: { type: Boolean, default: false },
})
defineEmits(['update:modelValue'])
</script>

<style scoped>
.checkbox-container {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  cursor: pointer;
  user-select: none;
}

.checkbox-container input {
  position: absolute;
  opacity: 0;
  cursor: pointer;
  height: 0;
  width: 0;
}

.checkmark {
  width: 20px;
  height: 20px;
  background-color: var(--surface-container-low);
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  border-radius: 6px;
  flex-shrink: 0;
  position: relative;
  margin-top: 2px;
  transition: all 0.2s ease;
}

.checkbox-container:hover input ~ .checkmark {
  background-color: var(--surface-container);
  border-color: color-mix(in srgb, var(--outline-variant) 40%, transparent);
}

.checkbox-container input:checked ~ .checkmark {
  background-color: var(--primary);
  border-color: var(--primary);
}

.checkmark:after {
  content: '';
  position: absolute;
  display: none;
}

.checkbox-container input:checked ~ .checkmark:after {
  display: block;
}

.checkbox-container .checkmark:after {
  left: 6px;
  top: 2px;
  width: 5px;
  height: 10px;
  border: solid white;
  border-width: 0 2px 2px 0;
  transform: rotate(45deg);
}

.check-label {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

label {
  font-weight: 600;
  font-size: 14px;
  color: var(--on-surface);
}
</style>
