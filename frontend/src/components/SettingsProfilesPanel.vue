<template>
  <article class="panel profiles-panel">
    <div class="panel-header">
      <h2>Profiles & Targets</h2>
      <button class="add-profile-btn" @click="$emit('add')">+ Create Profile</button>
    </div>

    <div v-if="profiles.length === 0" class="empty-state">
      No profiles defined. Create one to organize textbooks.
    </div>

    <div v-else class="profiles-list">
      <div
        v-for="profile in profiles"
        :key="profile.id"
        class="profile-card"
        :class="{ active: activeProfileId === profile.id }"
      >
        <div class="profile-info">
          <h3>{{ profile.name }}</h3>
          <p class="deadline">
            Deadline: <strong>{{ formatUnixDate(profile.deadline_at) }}</strong>
          </p>
        </div>
        <div class="profile-actions">
          <button
            v-if="activeProfileId !== profile.id"
            class="select-btn"
            @click="$emit('select', profile.id)"
          >
            Select Active
          </button>
          <button
            v-else
            class="deselect-btn"
            title="Click to deselect active profile"
            @click="$emit('select', '')"
          >
            Active ✓ (Deselect)
          </button>

          <button class="edit-btn" @click="$emit('edit', profile)">Edit</button>
          <button class="delete-btn" @click="$emit('delete', profile.id)">Delete</button>
        </div>
      </div>
    </div>
  </article>
</template>

<script setup>
defineProps({
  profiles: { type: Array, required: true },
  activeProfileId: { type: String, default: '' },
  formatUnixDate: { type: Function, required: true },
})

defineEmits(['add', 'select', 'edit', 'delete'])
</script>

<style scoped>
.profiles-panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 28px;
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  box-shadow: 0 4px 20px color-mix(in srgb, var(--on-surface) 3%, transparent);
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.panel-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
}

.add-profile-btn {
  background: var(--primary);
  color: var(--on-primary);
  border: none;
  border-radius: 8px;
  padding: 8px 16px;
  font-weight: 700;
  font-size: 13px;
  cursor: pointer;
}

.profiles-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.profile-card {
  border: 1px solid var(--outline-variant);
  border-radius: 12px;
  padding: 16px;
  background: var(--surface-container-low);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.profile-card.active {
  border-color: var(--primary);
  box-shadow: 0 0 10px rgba(108, 92, 231, 0.15);
}

.profile-info h3 {
  margin: 0 0 4px;
  font-size: 16px;
}

.profile-info .deadline {
  margin: 0;
  font-size: 13px;
  color: var(--muted-text);
}

.profile-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.select-btn {
  background: rgba(108, 92, 231, 0.1);
  color: var(--primary);
  border: none;
  border-radius: 8px;
  padding: 6px 12px;
  font-weight: 700;
  font-size: 12px;
  cursor: pointer;
}

.deselect-btn {
  background: var(--primary);
  color: var(--on-primary);
  border: none;
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
  transition: opacity 0.2s ease;
}

.deselect-btn:hover {
  opacity: 0.85;
}

.edit-btn,
.delete-btn {
  background: none;
  border: none;
  color: var(--muted-text);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 6px;
}

.edit-btn:hover {
  background: rgba(255, 255, 255, 0.05);
  color: var(--on-surface);
}

.delete-btn:hover {
  background: rgba(235, 94, 85, 0.1);
  color: #eb5e55;
}
</style>
