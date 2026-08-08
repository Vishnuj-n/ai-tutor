<template>
  <div class="assignments-page">
    <section class="section-card">
      <h2 class="section-title">
        Course Assignments
      </h2>

      <form @submit.prevent="publishAssignment" style="margin-bottom: 2rem;">
        <h3 style="margin-top: 0; font-size: 0.85rem; color: var(--on-surface); margin-bottom: 0.85rem; font-weight: 600; letter-spacing: 0.05em; text-transform: uppercase;">
          Publish New PDF
        </h3>

        <div class="form-group">
          <label for="assign-title">Assignment Title</label>
          <input
            id="assign-title"
            v-model="newTitle"
            type="text"
            required
            placeholder="e.g. Chapter 4: Cell Division"
          />
        </div>

        <div class="form-group">
          <label for="assign-file">Upload Local PDF File (Max 50MB)</label>
          <input
            id="assign-file"
            type="file"
            accept="application/pdf,.pdf"
            :disabled="uploadingPdf || publishing"
            @change="handleFileUpload"
          />
          <span v-if="uploadingPdf" class="muted" style="font-size: 0.8rem; margin-top: 0.25rem; display: block;">
            Uploading PDF to Supabase Storage...
          </span>
        </div>

        <div class="form-group">
          <label for="assign-url">Or Direct PDF URL</label>
          <input
            id="assign-url"
            v-model="newUrl"
            type="url"
            required
            placeholder="https://example.com/files/cell_chap4.pdf"
          />
        </div>

        <div style="display: flex; gap: 1rem;">
          <div class="form-group" style="flex: 1;">
            <label for="assign-start-page">Start Page (Optional)</label>
            <input
              id="assign-start-page"
              v-model.number="newStartPage"
              type="number"
              min="1"
              placeholder="e.g. 1"
            />
          </div>
          <div class="form-group" style="flex: 1;">
            <label for="assign-end-page">End Page (Optional)</label>
            <input
              id="assign-end-page"
              v-model.number="newEndPage"
              type="number"
              min="1"
              placeholder="e.g. 25"
            />
          </div>
        </div>

        <!-- Native HTML PDF Preview Drawer for Form Upload -->
        <div v-if="newUrl" class="pdf-preview-drawer" style="margin-top: 1rem;">
          <h4 style="font-size: 0.8rem; text-transform: uppercase; color: var(--muted-text); margin-bottom: 0.5rem;">
            PDF Preview (Page 1)
          </h4>
          <iframe
            :src="`${newUrl}#page=1`"
            style="width: 100%; height: 350px; border: 1px solid var(--border); border-radius: 8px; background: #fff;"
            title="PDF Preview"
          ></iframe>
        </div>

        <button class="btn" style="width: 100%; margin-top: 0.5rem;" :disabled="publishing">
          <span v-if="publishing" class="loading-spinner" style="width: 12px; height: 12px; border-width: 2px;"></span>
          {{ publishing ? 'Publishing...' : 'Publish to Class' }}
        </button>
      </form>

      <div>
        <h3 style="font-size: 0.85rem; color: var(--muted-text); margin-bottom: 0.85rem; border-top: 1px solid var(--border); padding-top: 1.25rem; font-weight: 600; letter-spacing: 0.05em; text-transform: uppercase;">
          Active Assignments ({{ assignments.length }})
        </h3>

        <div v-if="loadingAssignments" class="text-center" style="padding: 1.5rem 0;">
          <div class="loading-spinner"></div>
        </div>

        <div v-else-if="assignments.length === 0" class="muted" style="font-size: 0.8rem; font-style: italic; text-align: center; padding: 2rem 1rem; border: 1px dashed var(--border); border-radius: 8px; background: rgba(255,255,255,0.01);">
          No assignments published yet.
        </div>

        <div v-else class="assignments-list">
          <div
            v-for="asm in assignments"
            :key="asm.id"
            class="assignment-item"
          >
            <div class="assignment-info">
              <h4 class="assignment-title" :title="asm.title">
                PDF: {{ asm.title }}
                <span v-if="asm.start_page || asm.end_page" class="badge" style="font-size: 0.75rem; margin-left: 0.5rem; color: var(--accent);">
                  Pages {{ asm.start_page || 1 }}–{{ asm.end_page || 'End' }}
                </span>
              </h4>
              <a :href="asm.download_url" target="_blank" class="assignment-url" :title="asm.download_url">
                {{ asm.download_url }}
              </a>
              <span class="assignment-date">Published {{ formatDate(asm.created_at) }}</span>
            </div>
            <div style="display: flex; gap: 0.5rem;">
              <button
                class="btn btn-secondary"
                style="padding: 0.35rem 0.55rem; font-size: 0.75rem; border-radius: 6px; min-height: unset;"
                @click="previewAssignmentUrl = previewAssignmentUrl === asm.download_url ? null : asm.download_url"
              >
                {{ previewAssignmentUrl === asm.download_url ? 'Close Preview' : 'Preview' }}
              </button>
              <button
                class="btn btn-secondary btn-danger"
                style="padding: 0.35rem 0.55rem; font-size: 0.75rem; border-radius: 6px; min-height: unset;"
                @click="deleteAssignment(asm.id)"
                title="Remove assignment"
              >
                Delete
              </button>
            </div>
            <!-- Active Assignment PDF Preview Drawer -->
            <div v-if="previewAssignmentUrl === asm.download_url" style="width: 100%; margin-top: 0.75rem;">
              <iframe
                :src="`${asm.download_url}#page=${asm.start_page || 1}`"
                style="width: 100%; height: 350px; border: 1px solid var(--border); border-radius: 8px; background: #fff;"
                title="Assignment PDF Preview"
              ></iframe>
            </div>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { ref } from 'vue';
import { useDashboard } from '../composables/useDashboard';

const previewAssignmentUrl = ref(null);

const {
  newTitle,
  newUrl,
  newStartPage,
  newEndPage,
  publishing,
  uploadingPdf,
  assignments,
  loadingAssignments,
  handleFileUpload,
  publishAssignment,
  deleteAssignment,
  formatDate
} = useDashboard();
</script>
