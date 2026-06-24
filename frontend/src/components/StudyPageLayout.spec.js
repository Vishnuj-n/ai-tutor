import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import StudyPageLayout from './StudyPageLayout.vue'

describe('StudyPageLayout.vue', () => {
  it('renders required title and optional eyebrow/subtitle props', () => {
    const wrapper = mount(StudyPageLayout, {
      props: {
        title: 'Queue Dashboard',
        eyebrow: 'AI Tutor',
        subtitle: 'Review your study queue'
      }
    })
    expect(wrapper.find('.page-title').text()).toBe('Queue Dashboard')
    expect(wrapper.find('.eyebrow').text()).toBe('AI Tutor')
    expect(wrapper.find('.page-subtitle').text()).toBe('Review your study queue')
  })

  it('renders slot content and toolbar slot', () => {
    const wrapper = mount(StudyPageLayout, {
      props: {
        title: 'Test Title'
      },
      slots: {
        default: '<div class="main-content">Main Content</div>',
        toolbar: '<button class="toolbar-btn">Actions</button>'
      }
    })
    expect(wrapper.find('.main-content').text()).toBe('Main Content')
    expect(wrapper.find('.toolbar-btn').text()).toBe('Actions')
  })
})
