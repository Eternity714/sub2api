import { reactive, watchSyncEffect } from 'vue'
import { CONCRETE_PLATFORM_OPTIONS } from '@/constants/platforms'
import type { GroupPlatform } from '@/types'

export const channelPlatformOrder: GroupPlatform[] = reactive([])

watchSyncEffect(() => {
  const platforms = CONCRETE_PLATFORM_OPTIONS.map(option => option.value)
  channelPlatformOrder.splice(0, channelPlatformOrder.length, ...platforms)
})

export function selectableChannelPlatforms(_existingPlatforms: readonly GroupPlatform[] = []): GroupPlatform[] {
  return [...channelPlatformOrder]
}
