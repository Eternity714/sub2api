import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import PlatformIcon from '../PlatformIcon.vue'
import PlatformTypeBadge from '../PlatformTypeBadge.vue'

describe('media platform display', () => {
  it('renders public media as an image/video mark without a remote brand asset', () => {
    const wrapper = mount(PlatformIcon, { props: { platform: 'media' } })
    expect(wrapper.find('svg rect').exists()).toBe(true)
    expect(wrapper.find('svg circle').exists()).toBe(true)
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.html()).not.toMatch(/grsai|GRS\.AI/)
  })

  it('keeps the upstream provider brand visible on administrator account badges', () => {
    const wrapper = mount(PlatformTypeBadge, {
      props: { platform: 'grsai', type: 'apikey' },
      global: { plugins: [createI18n({ legacy: false, locale: 'en', messages: { en: { admin: { accounts: { apiKey: 'API Key' } } } } })] }
    })
    expect(wrapper.text()).toContain('GRS.AI')
    expect(wrapper.find('img[alt="GRS.AI"]').exists()).toBe(true)
  })
})
