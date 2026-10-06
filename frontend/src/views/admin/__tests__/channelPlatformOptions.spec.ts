import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { channelPlatformOrder, selectableChannelPlatforms } from '../channelPlatformOptions'

describe('Composite channel platform options', () => {
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
    const declaration = source.match(/const compositePlatforms:[^=]+=[^\n]+/)?.[0]

    expect(declaration).toContain("'kimi'")
    expect(declaration).toContain("'zhipu'")
    expect(declaration).toContain("'deepseek'")
    expect(declaration).toContain("'minimax'")
    expect(declaration).toContain("'opencode_go'")
  })
})
