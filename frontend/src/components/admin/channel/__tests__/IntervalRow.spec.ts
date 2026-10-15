import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import IntervalRow from '../IntervalRow.vue'
import type { IntervalFormEntry } from '../types'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
const interval: IntervalFormEntry = {
  min_tokens: 0, max_tokens: null, tier_label: '', sort_order: 0,
  input_price: null, output_price: null, cache_write_price: null, cache_read_price: null,
  input_multiplier: null, output_multiplier: null, cache_write_multiplier: null,
  cache_read_multiplier: null, per_request_price: null,
}

describe('pricing interval token bounds', () => {
  it.each([
    ['1e6', 1000000, 1000000],
    ['2.72e5', 272000, 272000],
    ['128000', 128000, 128000],
    ['', 0, null],
  ])('parses %s without losing the exponent', async (value, min, max) => {
    const wrapper = mount(IntervalRow, { props: { interval, mode: 'token' } })
    const inputs = wrapper.findAll('input[type="number"]')
    await inputs[0].setValue(value)
    await inputs[1].setValue(value)
    expect(wrapper.emitted('update')?.[0]?.[0]).toMatchObject({ min_tokens: min })
    expect(wrapper.emitted('update')?.[1]?.[0]).toMatchObject({ max_tokens: max })
  })
})

describe('media tier price units', () => {
  it.each([
    ['image', 'perImagePrice', 'priceUnitImage', '1K / 2K / 4K'],
    ['video', 'perSecondPrice', 'priceUnitSecond', '480p / 720p / 1080p'],
  ] as const)('edits %s prices without token bounds', async (mode, priceLabel, unitLabel, placeholder) => {
    const tier = { ...interval, tier_label: mode === 'video' ? '720p' : '2K', per_request_price: 0.35 }
    const wrapper = mount(IntervalRow, { props: { interval: tier, mode } })

    expect(wrapper.text()).toContain(`admin.channels.form.${priceLabel}`)
    expect(wrapper.text()).toContain(`admin.channels.form.${unitLabel}`)
    expect(wrapper.text()).toContain('admin.channels.form.resolution')
    expect(wrapper.text()).not.toContain('admin.channels.form.minTokens')
    expect(wrapper.get('input[type="text"]').attributes('placeholder')).toBe(placeholder)
    expect(wrapper.findAll('input[type="number"]')).toHaveLength(1)
    await wrapper.get('input[type="number"]').setValue('0.7007')
    expect(wrapper.emitted('update')?.[0]?.[0]).toEqual({ ...tier, per_request_price: '0.7007' })
  })

  it('keeps context bounds available for per-request tiers', () => {
    const wrapper = mount(IntervalRow, { props: { interval, mode: 'per_request' } })
    expect(wrapper.findAll('input[type="number"]')).toHaveLength(3)
    expect(wrapper.text()).toContain('admin.channels.form.priceUnitRequest')
  })
})
