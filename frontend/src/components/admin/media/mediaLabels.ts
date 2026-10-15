import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

/** Keep this small feature's bilingual labels together without changing shared business locales. */
export function useMediaLabels() {
  const { locale } = useI18n()
  const english = computed(() => locale?.value?.startsWith('en') ?? false)
  return (zh: string, en: string) => english.value ? en : zh
}
