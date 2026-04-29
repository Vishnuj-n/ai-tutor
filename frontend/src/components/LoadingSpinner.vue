<template>
  <span class="spinner"></span>
</template>

<script setup>
// No props needed for basic spinner
</script>

<style scoped>
.spinner {
  width: 16px;
  height: 16px;
  border: 2px solid transparent;
  border-top: 2px solid currentColor;
  border-radius: 50%;
  animation: spin 0.6s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
