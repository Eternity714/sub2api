import { describe, expect, it } from 'vitest'
import { COMPOSITE_ROUTE_PLATFORM_OPTIONS } from '@/constants/platforms'

describe('GroupsView Composite route options', () => {
  it('offers Kimi, Zhipu GLM, and DeepSeek as route targets', () => {
    expect(COMPOSITE_ROUTE_PLATFORM_OPTIONS.map((option) => option.value)).toEqual(
      expect.arrayContaining(['kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go'])
    )
  })

  it('keeps native media endpoints out of composite routes', () => {
    expect(COMPOSITE_ROUTE_PLATFORM_OPTIONS.map((option) => option.value)).not.toContain('grsai')
  })
})
