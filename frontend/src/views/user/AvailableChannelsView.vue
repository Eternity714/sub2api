<template>
  <AppLayout>
    <div class="min-w-0 space-y-5">
      <div class="flex items-start gap-3">
        <PlazaFilterBar
          class="min-w-0 flex-1"
          :platforms="platforms"
          :groups="groupOptions"
          :rates="rates"
          v-model:platform="selectedPlatform"
          v-model:group-id="selectedGroupId"
          v-model:rate="selectedRate"
          v-model:search="searchQuery"
          :search-placeholder="t('availableChannels.searchPlaceholder')"
        />
        <button
          type="button"
          @click="loadChannels"
          :disabled="loading"
          class="btn btn-secondary shrink-0"
          :title="t('common.refresh')"
          :aria-label="t('common.refresh')"
        >
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>
      <p class="flex items-start gap-1.5 text-xs leading-5 text-gray-500 dark:text-dark-400">
        <Icon name="infoCircle" size="xs" class="mt-1 shrink-0" />
        {{ t('availableChannels.listedPriceHint') }}
      </p>
      <div v-if="loading" class="flex min-h-[240px] items-center justify-center" role="status" :aria-label="t('modelPlaza.loading')">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-600/25 border-t-primary-600 dark:border-primary-400/25 dark:border-t-primary-400"></div>
      </div>
      <div v-else-if="filteredChannels.length" class="space-y-5">
        <AvailableChannelSection
          v-for="(channel, index) in filteredChannels"
          :key="`${channel.name}-${index}`"
          :channel="channel"
          :user-group-rates="userGroupRates"
        />
      </div>
      <div v-else class="rounded-2xl border border-dashed border-gray-300 px-5 py-12 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-dark-400">
        {{ filtersActive ? t('availableChannels.noSearchResult') : t('availableChannels.empty') }}
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import PlazaFilterBar from '@/components/modelPlaza/PlazaFilterBar.vue'
import AvailableChannelSection from '@/components/channels/AvailableChannelSection.vue'
import userChannelsAPI, { type UserAvailableChannel } from '@/api/channels'
import userGroupsAPI from '@/api/groups'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { publicPlatformId, publicPlatformLabel } from '@/utils/publicPlatform'

const { t } = useI18n()
const appStore = useAppStore()

const channels = ref<UserAvailableChannel[]>([])
const userGroupRates = ref<Record<number, number>>({})
const loading = ref(false)
const searchQuery = ref('')
const selectedPlatform = ref('all')
const selectedGroupId = ref<number | 'all'>('all')
const selectedRate = ref<number | 'all'>('all')

const platforms = computed(() => [...new Set(channels.value.flatMap(channel =>
  channel.platforms.map(section => publicPlatformId(section.platform))
))].sort())

const groupOptions = computed(() => {
  const options = new Map<number, { id: number; name: string; platform: string; platforms: string[]; rate: number }>()
  for (const channel of channels.value) {
    for (const section of channel.platforms) {
      const platform = publicPlatformId(section.platform)
      for (const group of section.groups) {
        const existing = options.get(group.id)
        if (existing) {
          if (!existing.platforms.includes(platform)) existing.platforms.push(platform)
        } else {
          options.set(group.id, {
            id: group.id, name: group.name, platform, platforms: [platform],
            rate: userGroupRates.value[group.id] ?? group.rate_multiplier
          })
        }
      }
    }
  }
  return [...options.values()]
})

const rates = computed(() => [...new Set(groupOptions.value.map(group => group.rate))].sort((a, b) => a - b))
const filtersActive = computed(() => Boolean(searchQuery.value.trim()) || selectedPlatform.value !== 'all' || selectedGroupId.value !== 'all' || selectedRate.value !== 'all')

watch([platforms, groupOptions, rates], () => {
  if (!platforms.value.includes(selectedPlatform.value)) selectedPlatform.value = 'all'
  if (!groupOptions.value.some(group => group.id === selectedGroupId.value)) selectedGroupId.value = 'all'
  if (selectedRate.value !== 'all' && !rates.value.includes(selectedRate.value)) selectedRate.value = 'all'
})

/**
 * 搜索过滤：
 * 平台、分组、倍率先限制可见范围，再搜索渠道/描述/平台/分组/模型。
 * 搜索仅命中模型名时只保留命中模型，保留渠道与平台层级。
 */
const filteredChannels = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return channels.value
    .map((ch) => {
      const nameHit = ch.name.toLowerCase().includes(q)
      const descHit = (ch.description || '').toLowerCase().includes(q)
      const matchingSections = ch.platforms.flatMap(section => {
        if (selectedPlatform.value !== 'all' && publicPlatformId(section.platform) !== selectedPlatform.value) return []
        const groups = section.groups.filter(group =>
          (selectedGroupId.value === 'all' || group.id === selectedGroupId.value) &&
          (selectedRate.value === 'all' || (userGroupRates.value[group.id] ?? group.rate_multiplier) === selectedRate.value)
        )
        if ((selectedGroupId.value !== 'all' || selectedRate.value !== 'all') && !groups.length) return []
        const sectionHit = !q || nameHit || descHit ||
          publicPlatformId(section.platform).toLowerCase().includes(q) ||
          publicPlatformLabel(section.platform, t('admin.groups.platforms.media')).toLowerCase().includes(q) ||
          groups.some(group => group.name.toLowerCase().includes(q))
        const models = sectionHit ? section.supported_models : section.supported_models.filter(model => model.name.toLowerCase().includes(q))
        return sectionHit || models.length ? [{ ...section, groups, supported_models: models }] : []
      })
      if (matchingSections.length === 0) return null
      return { ...ch, platforms: matchingSections }
    })
    .filter((ch): ch is UserAvailableChannel => ch !== null)
})

async function loadChannels() {
  loading.value = true
  try {
    // 渠道列表和用户专属倍率并发拉取。专属倍率失败不阻塞渠道展示——
    // 失败时只是无法渲染专属倍率角标，降级为仅显示默认倍率。
    const [list, rates] = await Promise.all([
      userChannelsAPI.getAvailable(),
      userGroupsAPI.getUserGroupRates().catch((err: unknown) => {
        console.error('Failed to load user group rates:', err)
        return {} as Record<number, number>
      }),
    ])
    channels.value = list
    userGroupRates.value = rates
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    loading.value = false
  }
}

onMounted(loadChannels)
</script>
