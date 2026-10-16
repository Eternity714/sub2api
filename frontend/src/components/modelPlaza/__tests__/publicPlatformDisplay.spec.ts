import { createPinia } from 'pinia'
import { isVNode, type VNode } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { ModelPlazaGroup } from '@/api/modelPlaza'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import ModelPlazaContent from '../ModelPlazaContent.vue'
import PlazaModelPricingTable from '../PlazaModelPricingTable.vue'

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))

function group(id: number, platform: string): ModelPlazaGroup {
  return {
    id, name: `Channel ${id}`, description: '', platform, subscription_type: 'standard',
    rate_multiplier: 1, peak_rate_enabled: false, peak_start: '', peak_end: '', peak_rate_multiplier: 1,
    is_exclusive: false, image_rate_independent: false, image_rate_multiplier: 1,
    video_rate_independent: false, video_rate_multiplier: 1, long_context_pricing_enabled: false,
    models: [{ name: `image-${id}`, platform, pricing: null, official_pricing: null }],
  }
}

function renderedRowKeys(node: VNode): PropertyKey[] {
  const keys: PropertyKey[] = node.type === 'tr' && node.key != null ? [node.key] : []
  if (Array.isArray(node.children)) {
    for (const child of node.children) {
      if (isVNode(child)) keys.push(...renderedRowKeys(child))
    }
  }
  return keys
}

describe('public model plaza media identity', () => {
  it('retains different prices for the same public media model with unique standard and time-period row keys', async () => {
    const warnings: string[] = []
    const models = [0.1, 0.2].map(price => ({
      name: 'public-image-model', platform: 'media', official_pricing: null,
      pricing: {
        billing_mode: 'image' as const, per_request_price: price, intervals: [],
        input_price: null, output_price: null, cache_write_price: null, cache_read_price: null,
        image_input_price: null, image_output_price: null,
      },
      time_pricing: { timezone: 'UTC', periods: [{ start_time: '00:00:00', end_time: '01:00:00', multiplier: 0.5 }] },
    }))
    const wrapper = mount(PlazaModelPricingTable, {
      props: { models, platform: 'media', rateMultiplier: 1 },
      global: { config: { warnHandler: message => warnings.push(message) } },
    })
    try {
      const keys = renderedRowKeys(wrapper.vm.$.subTree)
      expect(keys).toHaveLength(4)
      expect(new Set(keys).size).toBe(4)
      await wrapper.setProps({ models: [...models].reverse() })
      const rows = wrapper.findAll('tbody tr')
      expect(rows).toHaveLength(4)
      expect(rows[0].text()).toContain('$0.20')
      expect(rows[2].text()).toContain('$0.10')
      expect(warnings.some(message => /duplicate keys/i.test(message))).toBe(false)
    } finally {
      wrapper.unmount()
    }
  })

  it('combines media source filters and keeps both groups visible with generic media icons', async () => {
    const wrapper = mount(ModelPlazaContent, {
      props: { response: { description: '', groups: [group(1, 'grsai'), group(2, 'toapis')] }, loading: false },
      global: { plugins: [createPinia()] },
    })
    try {
      const mediaFilters = wrapper.findAll('button').filter(button => button.text() === 'admin.groups.platforms.media')
      expect(mediaFilters).toHaveLength(1)
      await mediaFilters[0].trigger('click')
      expect(wrapper.text()).toContain('Channel 1')
      expect(wrapper.text()).toContain('Channel 2')
      expect(wrapper.text()).not.toMatch(/grsai|toapis|GRS\.AI/i)
      expect(wrapper.find('img[src*="grsai"]').exists()).toBe(false)
      for (const icon of wrapper.findAllComponents(PlatformIcon)) {
        expect(icon.props('platform')).toBe('media')
      }
    } finally {
      wrapper.unmount()
    }
  })

  it.each(['grsai', 'toapis'])('hides %s on Composite model price badges', platform => {
    const wrapper = mount(PlazaModelPricingTable, {
      props: { models: group(1, platform).models, platform: 'composite', rateMultiplier: 1 },
    })
    expect(wrapper.text()).toContain('admin.groups.platforms.media')
    expect(wrapper.text()).not.toMatch(/grsai|toapis|GRS\.AI/i)
    wrapper.unmount()
  })
})
