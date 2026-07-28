import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import Dashboard from './Dashboard.vue'
import * as appApi from '../services/appApi'

const routeQuery = ref({})

// Mock services/appApi
// Dashboard.vue uses getDashboardOverview as the single data-loading call.
// Individual getTodayPlan/getProfiles/getUserSettings/getStreakState are imported
// but not called inside loadAgenda — they must still appear in the factory to
// satisfy Vitest 4 strict mock enforcement.
vi.mock('../services/appApi', () => ({
  getDashboardOverview: vi.fn(),
  getTodayPlan: vi.fn(),
  getProfiles: vi.fn(),
  getUserSettings: vi.fn(),
  updateUserSettings: vi.fn(),
  getProfileDailyPace: vi.fn(),
  retryFlashcardGeneration: vi.fn(),
  getAppEnv: vi.fn(),
  devForceSocraticRescue: vi.fn(),
  devForceFlashcardGenerate: vi.fn(),
  getNotebooks: vi.fn(),
  getStreakState: vi.fn(),
  getFlashcardDueTimeline: vi.fn(),
}))

// Mock vue-router hooks
vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: routeQuery.value,
  }),
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
  }),
}))

// Canonical overview response used by most tests
const baseSettings = {
  max_flashcards_per_session: 30,
  study_start_time: '17:00',
  study_end_time: '18:00',
  reminders_enabled: true,
  active_profile_id: 'prof-1',
  skip_to_reading_active: false,
  cloud_sync_url: '',
  cloud_api_token: '',
  theme: '',
  rag_enabled: false,
  rag_notebook_chapter: true,
  rag_entire_notebook: true,
  rag_queue_study: true,
  default_remedial_strategy: 'CLASSIC',
  classroom_code: '',
  analytics_enabled: false,
  anonymous_user_id: '',
}

function makeOverview(overrides = {}) {
  return {
    settings: { ...baseSettings, ...overrides.settings },
    profiles: { profiles: [{ id: 'prof-1', name: 'John Doe' }], ...overrides.profiles },
    today_plan: {
      tasks: [],
      due_review_cards: 0,
      total_due_review_cards: 0,
      active_notebook_count: 0,
      ...overrides.today_plan,
    },
    streak_state: {
      current_streak: 2,
      longest_streak: 5,
      active_dates: [],
      today_completed: false,
      ...overrides.streak_state,
    },
  }
}

describe('Dashboard.vue Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeQuery.value = {}

    appApi.getAppEnv.mockResolvedValue({ env: 'dev' })
    appApi.getProfileDailyPace.mockResolvedValue({ completed_today: 0, target_today: 10 })
    appApi.getFlashcardDueTimeline.mockResolvedValue({ timeline: [], error: null })
    appApi.updateUserSettings.mockResolvedValue({ error: null })
  })

  it('renders today tasks and study statistics correctly', async () => {
    appApi.getDashboardOverview.mockResolvedValue(
      makeOverview({
        today_plan: {
          tasks: [
            {
              id: 'task-1',
              task_type: 'READING',
              title: 'Introduction to Calculus',
              notebook_name: 'Calculus 1',
              start_page: 1,
              end_page: 15,
              action_type: 'start_reading',
            },
          ],
          due_review_cards: 5,
          total_due_review_cards: 5,
          active_notebook_count: 1,
        },
      })
    )

    const wrapper = mount(Dashboard)
    await flushPromises()

    expect(wrapper.find('.status-strip h1').text()).toBe("Today's Tasks")
    expect(wrapper.text()).toContain('Introduction to Calculus')
    expect(wrapper.find('.review-count').text()).toContain('5 cards due for review')
  })

  it('toggles escape hatch status when clicked', async () => {
    appApi.getDashboardOverview.mockResolvedValue(
      makeOverview({
        today_plan: { tasks: [], due_review_cards: 0 },
      })
    )

    const wrapper = mount(Dashboard)
    await flushPromises()

    const toggleBtn = wrapper.find('.escape-hatch-toggle')
    expect(toggleBtn.text()).toBe('Skip to Reading')

    await toggleBtn.trigger('click')
    expect(appApi.updateUserSettings).toHaveBeenCalledWith(
      30,
      '17:00',
      '18:00',
      true,
      'prof-1',
      true, // toggled from false → true
      '',
      '',
      '',
      false,
      true,
      true,
      true,
      'CLASSIC',
      '',
      false,
      ''
    )
  })

  it('displays concept rescue banner when socratic task is present', async () => {
    appApi.getDashboardOverview.mockResolvedValue(
      makeOverview({
        today_plan: {
          tasks: [
            {
              id: 'task-2',
              task_type: 'SOCRATIC_REMEDIAL',
              action_type: 'socratic_remedial',
            },
          ],
          due_review_cards: 0,
        },
      })
    )

    const wrapper = mount(Dashboard)
    await flushPromises()

    expect(wrapper.find('.rescue-banner').exists()).toBe(true)
    expect(wrapper.find('.rescue-title').text()).toBe('Concept Rescue Active')
  })
})
