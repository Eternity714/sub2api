import type { AccountPlatform, GroupPlatform } from '@/types'

export interface PlatformOption<T extends string = string> {
  value: T
  label: string
}

/**
 * Concrete upstream platforms available for administrator account creation.
 */
export const CONCRETE_PLATFORM_OPTIONS = [
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'antigravity', label: 'Antigravity' },
  { value: 'grok', label: 'Grok' },
  { value: 'kimi', label: 'Kimi' },
  { value: 'zhipu', label: 'Zhipu GLM' },
  { value: 'deepseek', label: 'DeepSeek' },
  { value: 'minimax', label: 'MiniMax' },
  { value: 'opencode_go', label: 'OpenCode' },
  { value: 'typesafe', label: 'TypeSafe / Jev' },
  { value: 'grsai', label: 'GRS.AI' }
] as const satisfies readonly PlatformOption<AccountPlatform>[]

/** Platforms available for new groups. */
export const GROUP_PLATFORM_OPTIONS = [
  ...CONCRETE_PLATFORM_OPTIONS.filter((option) => option.value !== 'grsai'),
  { value: 'grsai', label: 'Media API' },
  { value: 'composite', label: 'Composite' }
] as const satisfies readonly PlatformOption<GroupPlatform>[]

/** Native media uses its own endpoint and cannot target a composite route. */
export const COMPOSITE_ROUTE_PLATFORM_OPTIONS = CONCRETE_PLATFORM_OPTIONS.filter((option) => option.value !== 'grsai')
