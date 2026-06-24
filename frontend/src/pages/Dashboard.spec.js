import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import Dashboard from './Dashboard.vue'
import * as appApi from '../services/appApi'

const routeQuery = ref({})

// Mock services/appApi
vi.mock('../services/appApi', () => ({
  getTodayPlan: vi.fn(),
  getProfiles: vi.fn(),
  getUserSettings: vi.fn(),
  updateUserSettings: vi.fn(),
  getProfileDailyPace: vi.fn(),
  triggerCloudSync: vi.fn(),
  getAppEnv: vi.fn(),
  devForceSocraticRescue: vi.fn(),
  devForceFlashcardSync: vi.fn(),
  getNotebooks: vi.fn()
}))

// Mock vue-router hooks
vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: routeQuery.value
  }),
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn()
  })
}))

describe('Dashboard.vue Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeQuery.value = {}
    
    // Default mock setups to pass the initial onMounted flow
    appApi.getAppEnv.mockResolvedValue({ env: 'dev' })
    appApi.getProfiles.mockResolvedValue({ profiles: [{ id: 'prof-1', name: 'John Doe' }] })
    appApi.getUserSettings.mockResolvedValue({
      daily_study_minutes: 60,
      active_profile_id: 'prof-1',
      skip_to_reading_active: false
    })
    appApi.getProfileDailyPace.mockResolvedValue({ completed_today: 0, target_today: 10 })
  })

  it('renders today tasks and study statistics correctly', async () => {
    appApi.getTodayPlan.mockResolvedValue({
      tasks: [
        {
          id: 'task-1',
          task_type: 'READING',
          title: 'Introduction to Calculus',
          notebook_name: 'Calculus 1',
          start_page: 1,
          end_page: 15,
          action_type: 'start_reading'
        }
      ],
      due_review_cards: 5,
      active_notebook_count: 1
    })

    const wrapper = mount(Dashboard)
    await flushPromises()

    expect(wrapper.find('.status-strip h1').text()).toBe("Today's Tasks")
    expect(wrapper.text()).toContain('Introduction to Calculus')
    expect(wrapper.find('.review-count').text()).toContain('5 cards due for review')
  })

  it('toggles escape hatch status when clicked', async () => {
    appApi.getTodayPlan.mockResolvedValue({ tasks: [], due_review_cards: 0 })
    appApi.updateUserSettings.mockResolvedValue({ error: null })

    const wrapper = mount(Dashboard)
    await flushPromises()

    const toggleBtn = wrapper.find('.escape-hatch-toggle')
    expect(toggleBtn.text()).toBe('Skip to Reading')

    await toggleBtn.trigger('click')
    expect(appApi.updateUserSettings).toHaveBeenCalledWith(
      60,
      'prof-1',
      true,
      undefined,
      undefined,
      '',
      false
    )
  })

  it('displays concept rescue banner when socratic task is present', async () => {
    appApi.getTodayPlan.mockResolvedValue({
      tasks: [
        {
          id: 'task-2',
          task_type: 'SOCRATIC_REMEDIAL',
          action_type: 'socratic_remedial'
        }
      ],
      due_review_cards: 0
    })

    const wrapper = mount(Dashboard)
    await flushPromises()

    expect(wrapper.find('.rescue-banner').exists()).toBe(true)
    expect(wrapper.find('.rescue-title').text()).toBe('Concept Rescue Active')
  })
})
