import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import SocraticRescue from './SocraticRescue.vue'
import * as appApi from '../services/appApi'

const routeQuery = ref({})

// Mock services/appApi
vi.mock('../services/appApi', () => ({
  getTopicSectionsContent: vi.fn(),
  completeSocraticRescue: vi.fn(),
  GetTaskContext: vi.fn(),
  activateTask: vi.fn(),
}))

// Mock vue-router hooks
vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: routeQuery.value,
  }),
  useRouter: () => ({
    push: vi.fn(),
  }),
}))

describe('SocraticRescue.vue Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeQuery.value = {
      topicId: 'topic-123',
      taskId: 'task-456',
      startPage: '1',
      endPage: '10',
    }

    // Mock clipboard
    Object.assign(navigator, {
      clipboard: {
        writeText: vi.fn().mockResolvedValue(),
      },
    })

    // Mock GetTaskContext
    appApi.GetTaskContext.mockResolvedValue({
      task: {
        id: 'task-456',
        topic_id: 'topic-123',
        notebook_id: 'notebook-789',
        start_page: 1,
        end_page: 10,
      },
    })

    appApi.activateTask.mockResolvedValue({ error: null })
  })

  it('loads source material and displays generated socratic prompt', async () => {
    appApi.getTopicSectionsContent.mockResolvedValue({
      content: 'DeepMind builds AI agents.',
    })

    const wrapper = mount(SocraticRescue)
    await flushPromises()

    expect(appApi.GetTaskContext).toHaveBeenCalledWith('task-456')
    expect(appApi.activateTask).toHaveBeenCalledWith('task-456')
    expect(appApi.getTopicSectionsContent).toHaveBeenCalledWith('topic-123', 'notebook-789')
    expect(wrapper.find('.source-text').text()).toBe('DeepMind builds AI agents.')
    expect(wrapper.find('.prompt-textarea').element.value).toContain('DeepMind builds AI agents.')
  })

  it('completes socratic rescue session when user clicks I completed the session', async () => {
    appApi.getTopicSectionsContent.mockResolvedValue({ content: '' })
    appApi.completeSocraticRescue.mockResolvedValue({ error: null })

    const wrapper = mount(SocraticRescue)
    await flushPromises()

    const completeBtn = wrapper.find('.complete-btn')
    await completeBtn.trigger('click')
    await flushPromises()

    expect(appApi.completeSocraticRescue).toHaveBeenCalledWith('task-456')
  })

  it('shows error if missing required query params', async () => {
    routeQuery.value = {} // Empty query parameters

    const wrapper = mount(SocraticRescue)
    await flushPromises()

    expect(wrapper.find('.error-state').exists()).toBe(true)
    expect(wrapper.find('.error-msg').text()).toContain('Missing required route context')
  })
})
