<template>
  <section class="min-w-0 overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-card dark:border-dark-700/50 dark:bg-dark-800/50">
    <header class="border-b border-gray-100 px-5 py-4 dark:border-dark-700/60">
      <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ channel.name }}</h2>
      <p v-if="channel.description" class="mt-1.5 whitespace-pre-line break-words text-sm text-gray-500 dark:text-dark-400">{{ channel.description }}</p>
    </header>
    <div class="divide-y divide-gray-200 dark:divide-dark-700">
      <div v-for="(section, index) in channel.platforms" :key="`${section.platform}-${index}`" class="min-w-0">
        <div class="space-y-3 px-5 py-4">
          <span :class="['inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium', platformBadgeLightClass(publicPlatformId(section.platform))]">
            <PlatformIcon :platform="publicPlatformId(section.platform) as GroupPlatform" size="xs" />
            {{ publicPlatformLabel(section.platform, t('admin.groups.platforms.media')) }}
          </span>
          <div class="flex flex-wrap items-center gap-2">
            <span class="text-xs text-gray-400 dark:text-dark-500">{{ t('availableChannels.columns.groups') }}</span>
            <div v-for="group in section.groups" :key="group.id" class="flex max-w-full min-w-0 flex-wrap items-center gap-1.5">
              <GroupBadge
                class="max-w-full"
                :name="group.name"
                :platform="publicPlatformId(group.platform) as GroupPlatform"
                :subscription-type="(group.subscription_type || 'standard') as SubscriptionType"
                :rate-multiplier="group.rate_multiplier"
                :user-rate-multiplier="userGroupRates[group.id] ?? null"
                :peak-rate-enabled="group.peak_rate_enabled"
                :peak-start="group.peak_start"
                :peak-end="group.peak_end"
                :peak-rate-multiplier="group.peak_rate_multiplier"
                :truncate="false"
                always-show-rate
              />
              <span v-if="group.is_exclusive" class="inline-flex items-center gap-1 rounded-md bg-purple-50 px-2 py-0.5 text-xs text-purple-600 dark:bg-purple-900/20 dark:text-purple-400" :title="t('availableChannels.exclusiveTooltip')">
                <Icon name="shield" size="xs" />{{ t('modelPlaza.badges.exclusive') }}
              </span>
            </div>
            <span v-if="section.groups.length === 0" class="text-xs text-gray-400">—</span>
          </div>
        </div>
        <PlazaModelPricingTable
          v-if="section.supported_models.length"
          :models="section.supported_models.map(model => ({ ...model, official_pricing: null }))"
          :platform="section.platform"
          :rate-multiplier="1"
          price-mode="listed"
        />
        <p v-else class="px-5 py-8 text-center text-sm text-gray-400 dark:text-dark-500">{{ t('availableChannels.noModels') }}</p>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import GroupBadge from '@/components/common/GroupBadge.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import PlazaModelPricingTable from '@/components/modelPlaza/PlazaModelPricingTable.vue'
import { publicPlatformId, publicPlatformLabel } from '@/utils/publicPlatform'
import { platformBadgeLightClass } from '@/utils/platformColors'
import type { UserAvailableChannel } from '@/api/channels'
import type { GroupPlatform, SubscriptionType } from '@/types'

defineProps<{ channel: UserAvailableChannel; userGroupRates: Record<number, number> }>()
const { t } = useI18n()
</script>
