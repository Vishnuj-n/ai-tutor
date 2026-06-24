import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import Reader from './Reader.vue'
import * as appApi from '../services/appApi'

// Mock JSDOM missing browser features
global.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
window.HTMLElement.prototype.scrollIntoView = vi.fn()


const routeQuery = ref({})

// Mock services/appApi
vi.mock('../services/appApi', () => ({
  completeReading: vi.fn(),
  getUserSettings: vi.fn(),
  logFrontendEvent: vi.fn(),
  getNotebookTopicTree: vi.fn(),
  getReaderTopicBundle: vi.fn(),
  initializeReadingSession: vi.fn(),
  askReaderAI: vi.fn()
}))

// Mock VuePdfEmbed since we cannot load PDF canvas in JSDOM
vi.mock('vue-pdf-embed', () => ({
  default: {
    name: 'VuePdfEmbed',
    template: '<div class="mock-pdf-embed">Mock PDF Content</div>'
  }
}))

// Mock vue-router hooks
vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: routeQuery.value
  }),
  useRouter: () => ({
    push: vi.fn()
  })
}))

describe('Reader.vue Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeQuery.value = { taskId: 'task-read-456', notebookId: 'nb-1', topicId: 'topic-1' }

    // Mock settings
    appApi.getUserSettings.mockResolvedValue({
      rag_enabled: true
    })

    // Mock reading session init to return simple task context satisfying composable validation
    appApi.initializeReadingSession.mockResolvedValue({
      ok: true,
      task: {
        id: 'task-read-456',
        task_type: 'READING',
        notebook_id: 'nb-1',
        topic_id: 'topic-1',
        start_page: 1,
        end_page: 5
      },
      page_bounds: {
        start_page: 1,
        end_page: 5,
        current_page: 1
      },
      navigation: {
        some_state: {}
      },
      bundle: {
        topic_title: 'Intro to AI',
        notebook_url: 'http://localhost/test.pdf',
        file_type: 'pdf',
        page_count: 5,
        topic_start_page: 1,
        topic_end_page: 5,
        sections: []
      }
    })

    // Mock topic tree and bundle
    appApi.getNotebookTopicTree.mockResolvedValue([])
    appApi.getReaderTopicBundle.mockResolvedValue({
      topic_id: 'topic-1',
      title: 'Intro to AI',
      start_page: 1,
      end_page: 5
    })
  })

  it('initializes reading session and displays PDF viewer placeholder', async () => {
    const wrapper = mount(Reader)
    await flushPromises()

    expect(appApi.initializeReadingSession).toHaveBeenCalledWith('task-read-456', 'nb-1', 'topic-1', 0, 0)
    expect(wrapper.find('.mock-pdf-embed').exists()).toBe(true)
    expect(wrapper.find('button.primary').text()).toBe('Complete Session')
  })

  it('completes reading task when Complete Session is clicked', async () => {
    appApi.completeReading.mockResolvedValue({
      error: null,
      quiz_task_id: 'quiz-next-789'
    })

    const wrapper = mount(Reader)
    await flushPromises()

    const completeBtn = wrapper.find('button.primary')
    await completeBtn.trigger('click')
    await flushPromises()

    expect(appApi.completeReading).toHaveBeenCalledWith('task-read-456')
  })
})
