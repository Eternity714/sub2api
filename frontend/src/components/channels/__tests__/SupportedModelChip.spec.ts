import { nextTick } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SupportedModelChip from '../SupportedModelChip.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

describe('SupportedModelChip', () => {
  describe.each(['availableChannels.pricing', 'admin.availableChannels.pricing'])(
    'image pricing using %s translations',
    (pricingKeyPrefix) => {
      it.each([
        { name: 'gpt-image-2', perImagePrice: 0.06, imageTokenPrice: 0.00003, expected: '$0.06' },
        { name: 'nano-banana-fast', perImagePrice: 0.044, imageTokenPrice: null, expected: '$0.044' },
        { name: 'free-image', perImagePrice: 0, imageTokenPrice: 0.00003, expected: '$0' }
      ])('shows $name per-image price instead of its image-token price', async (fixture) => {
        const wrapper = mount(SupportedModelChip, {
          attachTo: document.body,
          props: {
            pricingKeyPrefix,
            model: {
              name: fixture.name,
              platform: 'media',
              pricing: {
                billing_mode: 'image',
                input_price: null,
                output_price: null,
                cache_write_price: null,
                cache_read_price: null,
                image_input_price: null,
                image_output_price: fixture.imageTokenPrice,
                per_request_price: fixture.perImagePrice,
                intervals: []
              }
            }
          }
        })
        try {
          await wrapper.find('[tabindex="0"]').trigger('mouseenter')
          await nextTick()
          const tooltip = document.body.querySelector('[role="tooltip"]')
          expect(tooltip?.textContent).toContain(`${pricingKeyPrefix}.imagePrice`)
          expect(tooltip?.textContent).toContain(`${fixture.expected} ${pricingKeyPrefix}.unitPerImage`)
          expect(tooltip?.textContent).not.toContain('$0.00003')
          expect(tooltip?.textContent).not.toContain(`${pricingKeyPrefix}.unitPerRequest`)
        } finally {
          wrapper.unmount()
        }
      })

      it('shows a missing per-image price explicitly without using image-token pricing', async () => {
        const wrapper = mount(SupportedModelChip, {
          attachTo: document.body,
          props: {
            pricingKeyPrefix,
            model: {
              name: 'image-without-per-image-price',
              platform: 'media',
              pricing: {
                billing_mode: 'image',
                input_price: null,
                output_price: null,
                cache_write_price: null,
                cache_read_price: null,
                image_input_price: null,
                image_output_price: 0.00003,
                per_request_price: null,
                intervals: []
              }
            }
          }
        })
        try {
          await wrapper.find('[tabindex="0"]').trigger('mouseenter')
          await nextTick()
          const tooltip = document.body.querySelector('[role="tooltip"]')
          expect(tooltip?.textContent).toContain(`${pricingKeyPrefix}.imagePrice-`)
          expect(tooltip?.textContent).not.toContain('$0.00003')
        } finally {
          wrapper.unmount()
        }
      })

      it('shows image tier prices per image when no flat price is configured', async () => {
        const wrapper = mount(SupportedModelChip, {
          attachTo: document.body,
          props: {
            pricingKeyPrefix,
            model: {
              name: 'tiered-image',
              platform: 'media',
              pricing: {
                billing_mode: 'image',
                input_price: null,
                output_price: null,
                cache_write_price: null,
                cache_read_price: null,
                image_input_price: null,
                image_output_price: null,
                per_request_price: null,
                intervals: [
                  { tier_label: '1K', min_tokens: 0, max_tokens: null, input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: 0 },
                  { tier_label: '2K', min_tokens: 0, max_tokens: null, input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: 0.09 }
                ]
              }
            }
          }
        })
        try {
          await wrapper.find('[tabindex="0"]').trigger('mouseenter')
          await nextTick()
          const tooltip = document.body.querySelector('[role="tooltip"]')
          expect(tooltip?.textContent).toContain(`1K$0 ${pricingKeyPrefix}.unitPerImage`)
          expect(tooltip?.textContent).toContain(`2K$0.09 ${pricingKeyPrefix}.unitPerImage`)
        } finally {
          wrapper.unmount()
        }
      })
    }
  )

  it.each(['availableChannels.pricing', 'admin.availableChannels.pricing'])(
    'shows video prices per second using %s translations',
    async (pricingKeyPrefix) => {
      const wrapper = mount(SupportedModelChip, {
        attachTo: document.body,
        props: {
          pricingKeyPrefix,
          model: {
            name: 'video-test',
            platform: '',
            pricing: {
              billing_mode: 'video',
              input_price: null,
              output_price: null,
              cache_write_price: null,
              cache_read_price: null,
              image_input_price: null,
              image_output_price: null,
              per_request_price: 0.05,
              intervals: [
                { tier_label: '480p', min_tokens: 0, max_tokens: null, input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: 0 },
                { tier_label: '720p', min_tokens: 0, max_tokens: null, input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: 0.12 }
              ]
            }
          }
        }
      })
      try {
        await wrapper.find('[tabindex="0"]').trigger('mouseenter')
        await nextTick()
        const tooltip = document.body.querySelector('[role="tooltip"]')
        expect(tooltip?.textContent).toContain(`${pricingKeyPrefix}.billingModeVideo`)
        expect(tooltip?.textContent).toContain(`${pricingKeyPrefix}.videoPrice`)
        expect(tooltip?.textContent).toContain(`$0.05 ${pricingKeyPrefix}.unitPerSecond`)
        expect(tooltip?.textContent).toContain('480p')
        expect(tooltip?.textContent).toContain(`$0 ${pricingKeyPrefix}.unitPerSecond`)
        expect(tooltip?.textContent).toContain('720p')
        expect(tooltip?.textContent).toContain(`$0.12 ${pricingKeyPrefix}.unitPerSecond`)
        const model = wrapper.props('model')
        await wrapper.setProps({
          model: { ...model, pricing: { ...model.pricing!, per_request_price: null } }
        })
        expect(tooltip?.textContent).not.toContain(`${pricingKeyPrefix}.videoPrice`)
        expect(tooltip?.textContent).toContain(`$0.12 ${pricingKeyPrefix}.unitPerSecond`)
        await wrapper.setProps({
          model: { ...model, pricing: { ...model.pricing!, per_request_price: 0 } }
        })
        expect(tooltip?.textContent).toContain(`${pricingKeyPrefix}.videoPrice`)
        expect(wrapper.findComponent({ name: 'PricingRow' }).text()).toContain(`$0 ${pricingKeyPrefix}.unitPerSecond`)
      } finally {
        wrapper.unmount()
      }
    }
  )

  it('仅配置区间倍率时按基础价展示 token 档位', async () => {
    const wrapper = mount(SupportedModelChip, {
      attachTo: document.body,
      props: {
        model: {
          name: 'gpt-test',
          platform: '',
          pricing: {
            billing_mode: 'token',
            input_price: 10e-6,
            output_price: 50e-6,
            cache_write_price: null,
            cache_read_price: null,
            image_input_price: null,
            image_output_price: null,
            per_request_price: null,
            intervals: [{
              min_tokens: 272000,
              max_tokens: null,
              input_price: null,
              output_price: null,
              cache_write_price: null,
              cache_read_price: null,
              input_multiplier: 2,
              output_multiplier: 1.5,
              per_request_price: null
            }]
          }
        },
        showPlatform: false
      }
    })

    await wrapper.find('[tabindex="0"]').trigger('mouseenter')
    await nextTick()

    expect(document.body.textContent).toContain('$20 / $75')
    wrapper.unmount()
  })
})
