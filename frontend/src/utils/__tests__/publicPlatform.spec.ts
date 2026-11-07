import { describe, expect, it } from 'vitest'
import { publicPlatformId, publicPlatformLabel } from '../publicPlatform'

describe('public media identity', () => {
  it.each(['grsai', 'toapis', 'kuaipao'])('projects %s to the public media capability', platform => {
    expect(publicPlatformId(platform)).toBe('media')
    expect(publicPlatformLabel(platform, '媒体 API')).toBe('媒体 API')
  })

  it('preserves ordinary OpenAI identity', () => {
    expect(publicPlatformId('openai')).toBe('openai')
    expect(publicPlatformLabel('openai', '媒体 API', () => 'OpenAI')).toBe('OpenAI')
  })
})
