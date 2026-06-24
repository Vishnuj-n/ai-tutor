import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import BaseButton from './BaseButton.vue'

describe('BaseButton.vue', () => {
  it('renders slot content', () => {
    const wrapper = mount(BaseButton, {
      slots: {
        default: 'Click Me',
      },
    })
    expect(wrapper.text()).toContain('Click Me')
  })

  it('emits click event when clicked', async () => {
    const wrapper = mount(BaseButton)
    await wrapper.trigger('click')
    expect(wrapper.emitted()).toHaveProperty('click')
  })

  it('does not emit click event when disabled', async () => {
    const wrapper = mount(BaseButton, {
      props: {
        disabled: true,
      },
    })
    await wrapper.trigger('click')
    expect(wrapper.emitted('click')).toBeUndefined()
  })

  it('shows loading spinner and hides content when loading', () => {
    const wrapper = mount(BaseButton, {
      props: {
        loading: true,
      },
      slots: {
        default: 'Submit',
      },
    })
    expect(wrapper.find('.spinner').exists()).toBe(true)
    expect(wrapper.find('.btn-content').classes()).toContain('visually-hidden')
    expect(wrapper.element.disabled).toBe(true)
  })
})
