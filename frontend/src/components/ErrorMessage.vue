<template>
  <div v-if="message" class="error-msg" role="alert" aria-live="assertive" aria-atomic="true">
    {{ message }}
  </div>
</template>

<script setup>
defineProps({
  message: {
    type: String,
    default: ''
  }
})
</script>

<style scoped>
.error-msg {
  font-family: 'Inter', sans-serif;
  font-size: 0.9rem;
  font-weight: 500;
  color: #9f403d;
  padding: 1rem;
  background: rgba(159, 64, 61, 0.08);
  border-radius: 0.5rem;
  margin-top: 1rem;
}
</style>
