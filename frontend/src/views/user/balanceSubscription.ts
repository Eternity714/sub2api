import type { BalanceSubscriptionPurchaseResult } from '@/types/payment'

const INTENT_STORAGE_KEY = 'sub2api_balance_subscription_intent'
const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i

export interface BalanceSubscriptionIntent {
  userId: number
  planId: number
  key: string
  unresolved: boolean
  expectedAmount?: number
}

export function readBalanceSubscriptionIntent(userId: number, planId?: number): BalanceSubscriptionIntent | null {
  try {
    const raw = window.sessionStorage.getItem(`${INTENT_STORAGE_KEY}_${userId}`)
      || window.sessionStorage.getItem(INTENT_STORAGE_KEY)
    const intent = JSON.parse(raw || 'null') as BalanceSubscriptionIntent | null
    if (intent?.userId !== userId || !Number.isInteger(intent.planId) || intent.planId <= 0
      || (planId !== undefined && intent.planId !== planId) || !UUID_PATTERN.test(intent.key)) return null
    const expectedAmount = typeof intent.expectedAmount === 'number' && Number.isFinite(intent.expectedAmount) && intent.expectedAmount > 0
      ? intent.expectedAmount : undefined
    return { ...intent, expectedAmount, unresolved: intent.unresolved !== false }
  } catch {
    return null
  }
}

export function saveBalanceSubscriptionIntent(intent: BalanceSubscriptionIntent): void {
  try {
    window.sessionStorage.setItem(`${INTENT_STORAGE_KEY}_${intent.userId}`, JSON.stringify(intent))
  } catch {
    // Keep the in-memory key when browser storage is unavailable.
  }
}

export function createBalanceSubscriptionIntent(userId: number, planId: number, expectedAmount: number): BalanceSubscriptionIntent {
  let key: string
  if (globalThis.crypto.randomUUID) {
    key = globalThis.crypto.randomUUID()
  } else {
    const bytes = globalThis.crypto.getRandomValues(new Uint8Array(16))
    bytes[6] = (bytes[6] & 0x0f) | 0x40
    bytes[8] = (bytes[8] & 0x3f) | 0x80
    const hex = Array.from(bytes, value => value.toString(16).padStart(2, '0')).join('')
    key = `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
  }
  const intent = { userId, planId, key, unresolved: false, expectedAmount }
  saveBalanceSubscriptionIntent(intent)
  return intent
}

export function clearBalanceSubscriptionIntent(userId: number): void {
  try {
    window.sessionStorage.removeItem(`${INTENT_STORAGE_KEY}_${userId}`)
    const legacy = JSON.parse(window.sessionStorage.getItem(INTENT_STORAGE_KEY) || 'null') as BalanceSubscriptionIntent | null
    if (legacy?.userId === userId) window.sessionStorage.removeItem(INTENT_STORAGE_KEY)
  } catch {
    // The in-memory intent is still cleared by the caller.
  }
}

export function isCompletedBalanceSubscription(value: unknown): value is BalanceSubscriptionPurchaseResult {
  if (!value || typeof value !== 'object') return false
  const result = value as Record<string, unknown>
  const positiveNumber = (value: unknown): value is number => typeof value === 'number' && Number.isFinite(value) && value > 0
  return result.status === 'COMPLETED' && result.payment_type === 'balance' && result.currency === 'USD'
    && positiveNumber(result.order_id) && positiveNumber(result.subscription_id)
    && positiveNumber(result.amount) && positiveNumber(result.pay_amount)
    && typeof result.balance === 'number' && Number.isFinite(result.balance) && result.balance >= 0
}
