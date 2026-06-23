import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import SocraticRescue from './SocraticRescue.vue'
import * as appApi from '../services/appApi'

const routeQuery = ref({})

// Mock services/appApi
vi.mock('../services/appApi', () => ({
  getReaderTopicBundle: vi.fn(),
  completeSocraticRescue: vi.fn()
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

describe('SocraticRescue.vue Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeQuery.value = {
      topicId: 'topic-123',
      taskId: 'task-456',
      startPage: '1',
      endPage: '10'
    }

    // Mock clipboard
    Object.assign(navigator, {
      clipboard: {
        writeText: vi.fn().mockResolvedValue()
      }
    })
  })

  it('loads source material and displays generated socratic prompt', async () => {
    appApi.getReaderTopicBundle.mockResolvedValue({
      sections: [
        { content: 'DeepMind builds AI agents.' }
      ]
    })

    const wrapper = mount(SocraticRescue)
    await vi.dynamicImportSettled()

    expect(appApi.getReaderTopicBundle).toHaveBeenCalledWith('topic-123', '')
    expect(wrapper.find('.source-text').text()).toBe('DeepMind builds AI agents.')
    expect(wrapper.find('.prompt-textarea').element.value).toContain('DeepMind builds AI agents.')
  })

  it('completes socratic rescue session when user clicks I completed the session', async () => {
    appApi.getReaderTopicBundle.mockResolvedValue({ sections: [] })
    appApi.completeSocraticRescue.mockResolvedValue({ error: null })

    const wrapper = mount(SocraticRescue)
    await vi.dynamicImportSettled()

    const completeBtn = wrapper.find('.complete-btn')
    await completeBtn.trigger('click')
    await vi.dynamicImportSettled()

    expect(appApi.completeSocraticRescue).toHaveBeenCalledWith('task-456')
  })

  it('shows error if missing required query params', async () => {
    routeQuery.value = {} // Empty query parameters

    const wrapper = mount(SocraticRescue)
    await vi.dynamicImportSettled()

    expect(wrapper.find('.error-state').exists()).toBe(true)
    expect(wrapper.find('.error-msg').text()).toContain('Missing required route context')
  })
})
