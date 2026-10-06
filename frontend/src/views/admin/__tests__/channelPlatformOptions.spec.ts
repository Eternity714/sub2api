import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { BUILTIN_PLATFORM_CATALOG, listPlatformIds, resetPlatformCatalog, setPlatformCatalog } from '@/constants/platformCatalog'
import { COMPOSITE_ROUTE_PLATFORM_OPTIONS } from '@/constants/platforms'
import { channelPlatformOrder, selectableChannelPlatforms } from '../channelPlatformOptions'

describe('Composite channel platform options', () => {
  afterEach(() => resetPlatformCatalog())

  it('offers native media pricing for new channels', () => {
    expect(selectableChannelPlatforms()).toContain('openai')
    expect(selectableChannelPlatforms()).toContain('grsai')
  })

  it('offers a single media section for existing channels', () => {
    expect(channelPlatformOrder).toContain('grsai')
    expect(selectableChannelPlatforms(['grsai'])).toContain('grsai')
    expect(selectableChannelPlatforms(['openai'])).toContain('grsai')
    expect(channelPlatformOrder.filter((platform) => platform === 'grsai')).toHaveLength(1)
  })

  it('includes the CN concrete providers for pricing and model mapping', () => {
    const source = readFileSync(resolve('src/views/admin/ChannelsView.vue'), 'utf8')
    // 平台列表来自平台清单（后端 domain/platforms.go），组合分组排除原生媒体平台。
    expect(source).toMatch(/const platformOrder = computed<GroupPlatform\[\]>\(\(\) => listPlatformIds\(\)\)/)
    expect(source).toContain("const compositePlatforms = computed(() => platformOrder.value.filter(platform => platform !== 'grsai'))")

    expect(listPlatformIds()).toEqual(
      expect.arrayContaining(['kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go'])
    )
    expect(COMPOSITE_ROUTE_PLATFORM_OPTIONS.map(option => option.value)).not.toContain('grsai')
  })

  it('updates channel and composite options when the platform catalog changes', () => {
    setPlatformCatalog({
      platforms: [
        ...BUILTIN_PLATFORM_CATALOG.platforms,
        { id: 'acme_router', display_name: 'Acme Router', gateway: 'openai', cn_provider: false }
      ],
      composite_precedence: [...BUILTIN_PLATFORM_CATALOG.composite_precedence, 'acme_router']
    })

    expect(channelPlatformOrder).toContain('acme_router')
    expect(selectableChannelPlatforms()).toContain('acme_router')
    expect(COMPOSITE_ROUTE_PLATFORM_OPTIONS.map(option => option.value)).toContain('acme_router')
    expect(COMPOSITE_ROUTE_PLATFORM_OPTIONS.map(option => option.value)).not.toContain('grsai')
  })
})
