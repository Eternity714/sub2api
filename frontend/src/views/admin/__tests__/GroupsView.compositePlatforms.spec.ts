import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { CONCRETE_PLATFORM_OPTIONS } from '@/constants/platforms'

describe('GroupsView Composite route options', () => {
  it('offers Kimi, Zhipu GLM, and DeepSeek as route targets', () => {
    expect(CONCRETE_PLATFORM_OPTIONS.map((option) => option.value)).toEqual(
      expect.arrayContaining(['kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go'])
    )
  })

  it('excludes GRS.AI until its native route supports composite groups', () => {
    expect(CONCRETE_PLATFORM_OPTIONS.map((option) => option.value)).toContain('grsai')
    const source = readFileSync(resolve('src/views/admin/GroupsView.vue'), 'utf8')
    expect(source).toContain("CONCRETE_PLATFORM_OPTIONS.filter((option) => option.value !== 'grsai')")
  })
})
