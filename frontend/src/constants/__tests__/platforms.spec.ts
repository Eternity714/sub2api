import { describe, expect, it } from 'vitest'
import { CONCRETE_PLATFORM_OPTIONS, GROUP_PLATFORM_OPTIONS } from '@/constants/platforms'

const concretePlatforms = [
  'anthropic',
  'openai',
  'gemini',
  'antigravity',
  'grok',
  'kimi',
  'zhipu',
  'deepseek',
  'minimax',
  'opencode_go',
  'typesafe',
  'grsai'
]

describe('platform option catalogs', () => {
  it('exposes the supported account creation platforms', () => {
    expect(CONCRETE_PLATFORM_OPTIONS.map((option) => option.value)).toEqual(concretePlatforms)
  })

  it('uses the provider brand for accounts and the media capability for groups', () => {
    expect(CONCRETE_PLATFORM_OPTIONS.find((option) => option.value === 'grsai')?.label).toBe('GRS.AI')
    expect(GROUP_PLATFORM_OPTIONS.find((option) => option.value === 'grsai')?.label).toBe('Media API')
  })

  it('adds composite for group-backed filters', () => {
    expect(GROUP_PLATFORM_OPTIONS.map((option) => option.value)).toEqual([
      ...concretePlatforms,
      'composite'
    ])
  })
})
