import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import Notebook from './Notebook.vue'
import * as appApi from '../services/appApi'

// Mock Wails runtime
window.runtime = {
  EventsOn: vi.fn(),
  EventsOnMultiple: vi.fn(),
  EventsOff: vi.fn(),
}

vi.mock('../services/appApi', () => ({
  getNotebooks: vi.fn(),
  confirmNotebookSyllabus: vi.fn(),
  aiCleanupNotebookSyllabus: vi.fn(),
  getNotebookDetail: vi.fn(),
  updateNotebookPriority: vi.fn(),
  updateNotebookTitle: vi.fn(),
  deleteNotebook: vi.fn(),
  uploadNotebookPdf: vi.fn(),
  getNotebookPDFData: vi.fn(),
  getNotebookOutline: vi.fn(),
  updateNotebookChapterRange: vi.fn(),
  resetNotebookIngestionStatus: vi.fn(),
  getTodayPlan: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({
    params: {},
    query: {},
  }),
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
  }),
}))

describe('Notebook.vue - AI Cleanup then Confirm regression test', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    appApi.getNotebooks.mockResolvedValue([
      {
        id: 'nb-1',
        title: 'Test Notebook',
        priority: 1,
        page_count: 10,
        chunk_count: 5,
        file_type: 'pdf',
        indexing_status: 'READY',
        ingestion_status: 'READY',
        chapters: [{ title: 'Chapter 1 Raw', start_page: 1, end_page: 10 }],
      },
    ])
  })

  it('preserves originalDraftChapters snapshot after AI cleanup so confirmNotebookSyllabus receives cleaned chapters', async () => {
    appApi.aiCleanupNotebookSyllabus.mockResolvedValue({
      chapters: [
        { title: 'Cleaned Chapter 1', start_page: 1, end_page: 5 },
        { title: 'Cleaned Chapter 2', start_page: 6, end_page: 10 },
      ],
      page_count: 10,
    })
    appApi.confirmNotebookSyllabus.mockResolvedValue({ success: true })

    const wrapper = mount(Notebook, {
      global: {
        stubs: {
          RouterLink: true,
          SyllabusModal: true,
        },
      },
    })
    await flushPromises()

    // Open syllabus modal for draft
    wrapper.vm.draftNotebookID = 'nb-1'
    wrapper.vm.draftNotebookTitle = 'Test Notebook'
    wrapper.vm.draftNotebookPriority = 1
    wrapper.vm.originalDraftTitle = 'Test Notebook'
    wrapper.vm.originalDraftPriority = 1
    wrapper.vm.draftChapters = [{ title: 'Chapter 1 Raw', start_page: 1, end_page: 10 }]
    wrapper.vm.originalDraftChapters = [{ title: 'Chapter 1 Raw', start_page: 1, end_page: 10 }]

    // Trigger AI cleanup directly
    await wrapper.vm.aiCleanupChapters()
    await flushPromises()

    // Assert AI cleanup ran and updated draftChapters
    expect(appApi.aiCleanupNotebookSyllabus).toHaveBeenCalledWith('nb-1')
    expect(wrapper.vm.draftChapters).toEqual([
      { title: 'Cleaned Chapter 1', start_page: 1, end_page: 5 },
      { title: 'Cleaned Chapter 2', start_page: 6, end_page: 10 },
    ])

    // Assert originalDraftChapters snapshot remained unchanged
    expect(wrapper.vm.originalDraftChapters).toEqual([
      { title: 'Chapter 1 Raw', start_page: 1, end_page: 10 },
    ])

    // Perform confirm
    await wrapper.vm.handleConfirmSyllabus({
      title: 'Test Notebook',
      priority: 1,
      chapters: wrapper.vm.draftChapters,
    })
    await flushPromises()

    // Assert confirmNotebookSyllabus receives the cleaned chapters
    expect(appApi.confirmNotebookSyllabus).toHaveBeenCalledWith(
      'nb-1',
      [
        { title: 'Cleaned Chapter 1', start_page: 1, end_page: 5 },
        { title: 'Cleaned Chapter 2', start_page: 6, end_page: 10 },
      ]
    )
  })
})
