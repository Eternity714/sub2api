import { describe, expect, it } from 'vitest'
import { COMPOSITE_ROUTE_PLATFORM_OPTIONS } from '@/constants/platforms'

describe('GroupsView Composite route options', () => {
  it('offers Kimi, Zhipu GLM, and DeepSeek as route targets', () => {
    expect(COMPOSITE_ROUTE_PLATFORM_OPTIONS.map((option) => option.value)).toEqual(
      expect.arrayContaining(['kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go'])
    )
  })

  it('allows GRS.AI and ToAPIs as media composite route candidates', () => {
    expect(COMPOSITE_ROUTE_PLATFORM_OPTIONS.map((option) => option.value)).toEqual(expect.arrayContaining(['grsai', 'toapis']))
  })
})
