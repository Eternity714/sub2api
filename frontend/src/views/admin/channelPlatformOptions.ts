import { CONCRETE_PLATFORM_OPTIONS } from '@/constants/platforms'
import type { GroupPlatform } from '@/types'

export const channelPlatformOrder: GroupPlatform[] = CONCRETE_PLATFORM_OPTIONS.map((option) => option.value)

export function selectableChannelPlatforms(_existingPlatforms: readonly GroupPlatform[] = []): GroupPlatform[] {
  return [...channelPlatformOrder]
}
