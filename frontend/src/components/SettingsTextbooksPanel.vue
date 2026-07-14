<template>
  <article class="panel textbooks-panel">
    <h2>Textbook Assignments</h2>
    <p class="description">
      Assign uploaded textbooks to study profiles to calculate target deadlines.
    </p>

    <div v-if="notebooks.length === 0" class="empty-state">
      No textbooks uploaded. Go to Bookshelf to add them.
    </div>

    <table v-else class="textbooks-table">
      <thead>
        <tr>
          <th>Textbook Title</th>
          <th>Assigned Profile</th>
          <th>Status</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="nb in notebooks" :key="nb.id">
          <td class="nb-title">{{ nb.title }}</td>
          <td>
            <select
              :value="nb.profile_id || ''"
              class="profile-select"
              @change="$emit('assign', nb.id, $event.target.value)"
            >
              <option value="">-- Unassigned --</option>
              <option v-for="p in profiles" :key="p.id" :value="p.id">
                {{ p.name }}
              </option>
            </select>
          </td>
          <td>
            <span class="status-chip" :class="nb.study_status || 'dormant'">
              {{ nb.study_status || 'dormant' }}
            </span>
          </td>
        </tr>
      </tbody>
    </table>
  </article>
</template>

<script setup>
defineProps({
  notebooks: { type: Array, required: true },
  profiles: { type: Array, required: true },
})

defineEmits(['assign'])
</script>

<style scoped>
.textbooks-panel {
  background: var(--surface-container-lowest);
  border-radius: 16px;
  padding: 28px;
  border: 1px solid color-mix(in srgb, var(--outline-variant) 20%, transparent);
  box-shadow: 0 4px 20px color-mix(in srgb, var(--on-surface) 3%, transparent);
}

h2 {
  font-size: 20px;
  margin: 0 0 16px;
  font-weight: 700;
}

.description {
  margin: -12px 0 0;
  font-size: 13px;
  color: var(--muted-text);
  line-height: 1.4;
}

.textbooks-table {
  width: 100%;
  border-collapse: collapse;
  margin-top: 16px;
}

.textbooks-table th {
  text-align: left;
  padding: 10px 12px;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  color: var(--muted-text);
  border-bottom: 1px solid var(--outline-variant);
}

.textbooks-table td {
  padding: 12px;
  border-bottom: 1px solid var(--outline-variant);
  font-size: 14px;
}

.nb-title {
  font-weight: 600;
}

.profile-select {
  padding: 6px 10px;
  font-size: 13px;
  border-radius: 8px;
  width: 100%;
}

.status-chip {
  padding: 4px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.status-chip.active {
  background: rgba(37, 111, 54, 0.1);
  color: #256f36;
}

.status-chip.dormant {
  background: rgba(138, 139, 152, 0.1);
  color: var(--muted-text);
}

.status-chip.completed {
  background: rgba(108, 92, 231, 0.1);
  color: var(--primary);
}
</style>
