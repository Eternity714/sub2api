import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import PaymentView from '../PaymentView.vue'
import PaymentMethodSelector from '@/components/payment/PaymentMethodSelector.vue'
import SubscriptionPlanCard from '@/components/payment/SubscriptionPlanCard.vue'
import AmountInput from '@/components/payment/AmountInput.vue'
import type { CheckoutInfoResponse } from '@/types/payment'

const mocks = vi.hoisted(() => ({
  getCheckoutInfo: vi.fn(), purchaseSubscriptionWithBalance: vi.fn(), createOrder: vi.fn(),
  refreshUser: vi.fn(), fetchActiveSubscriptions: vi.fn(), showError: vi.fn(), showWarning: vi.fn(),
  routerPush: vi.fn(), setUser: (_user: { id: number; username: string; balance: number }) => {},
  route: { query: { tab: 'subscription', group: '3' } as Record<string, string> },
}))
vi.mock('vue-router', async () => ({
  ...await vi.importActual<typeof import('vue-router')>('vue-router'),
  useRoute: () => mocks.route,
  useRouter: () => ({ push: mocks.routerPush, replace: vi.fn(), resolve: vi.fn(() => ({ href: '/payment/stripe' })) }),
}))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ locale: 'en', t: (key: string) => key }),
}))
vi.mock('@/stores/auth', async () => {
  const { reactive } = await import('vue')
  const state = reactive({ user: { id: 9, username: 'demo', balance: 30 } })
  mocks.setUser = user => { state.user = user }
  return { useAuthStore: () => ({ get user() { return state.user }, refreshUser: mocks.refreshUser }) }
})
vi.mock('@/stores/payment', () => ({ usePaymentStore: () => ({ createOrder: mocks.createOrder }) }))
vi.mock('@/stores/subscriptions', () => ({ useSubscriptionStore: () => ({ activeSubscriptions: [], fetchActiveSubscriptions: mocks.fetchActiveSubscriptions }) }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ cachedPublicSettings: { subscription_enabled: true }, showError: mocks.showError, showWarning: mocks.showWarning, showInfo: vi.fn() }) }))
vi.mock('@/api/payment', () => ({ paymentAPI: { getCheckoutInfo: mocks.getCheckoutInfo, purchaseSubscriptionWithBalance: mocks.purchaseSubscriptionWithBalance } }))
vi.mock('@/utils/device', () => ({ isMobileDevice: () => false }))

function checkout(overrides: Partial<CheckoutInfoResponse> = {}) {
  return {
    methods: {}, global_min: 0, global_max: 0, balance_disabled: false,
    balance_recharge_multiplier: 0.14, subscription_usd_to_cny_rate: 7.15,
    recharge_fee_rate: 10, recharge_bonus_tiers: [{ min_amount: 1, bonus_percent: 50 }],
    recharge_bonus_mode: 'discount', help_text: '', help_image_url: '', stripe_publishable_key: '',
    subscription_balance_enabled: true,
    plans: [{ id: 7, group_id: 3, name: 'Starter', description: '', price: 10, currency: 'NZD', validity_days: 30, validity_unit: 'day', features: [], for_sale: true, sort_order: 0, group_platform: 'openai' }],
    ...overrides,
  }
}
function completed(overrides = {}) {
  return { data: { order_id: 42, status: 'COMPLETED', payment_type: 'balance', currency: 'USD', amount: 10, pay_amount: 10, balance: 20, subscription_id: 13, ...overrides } }
}
async function mountPage(overrides: Partial<CheckoutInfoResponse> = {}) {
  mocks.getCheckoutInfo.mockResolvedValue({ data: checkout(overrides) })
  const wrapper = shallowMount(PaymentView, {
    global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, PaymentMethodSelector: false, Teleport: true, Transition: false } },
  })
  await flushPromises()
  return wrapper
}
function selectBalance(wrapper: Awaited<ReturnType<typeof mountPage>>) {
  wrapper.getComponent(PaymentMethodSelector).vm.$emit('select', 'balance')
}

describe('subscription balance payment', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.refreshUser.mockResolvedValue({})
    mocks.fetchActiveSubscriptions.mockResolvedValue([])
    mocks.setUser({ id: 9, username: 'demo', balance: 30 })
    mocks.route.query = { tab: 'subscription', group: '3' }
    window.localStorage.clear()
    window.sessionStorage.clear()
  })
  afterEach(() => { vi.restoreAllMocks() })

  // AC-031.1: the shared CDK entry keeps the current page open on either checkout tab.
  it('keeps the CDK link visible on both tabs even without external methods', async () => {
    mocks.route.query = { tab: 'recharge' }
    const wrapper = await mountPage({ subscription_balance_enabled: false })
    const link = wrapper.get('[data-testid="purchase-cdk"]')
    expect(link.element.tagName).toBe('A')
    expect(link.attributes()).toMatchObject({ href: 'https://catfk.com/shop/BK9HYWAR', target: '_blank', rel: 'noopener noreferrer' })
    expect(link.text()).toBe('nav.cdkPurchase')
    await wrapper.findAll('button').find(button => button.text() === 'payment.tabSubscribe')!.trigger('click')
    expect(wrapper.get('[data-testid="purchase-cdk"]').attributes('target')).toBe('_blank')
  })

  // AC-031.2, AC-031.11: old servers opt out of the new capability.
  it('does not offer balance payments when the new capability is missing or disabled', async () => {
    for (const enabled of [undefined, false]) {
      const wrapper = await mountPage({ subscription_balance_enabled: enabled })
      expect(wrapper.find('[data-testid="balance-subscription-quote"]').exists()).toBe(false)
      expect(wrapper.find('[data-testid="confirm-balance-subscription"]').exists()).toBe(false)
      expect(mocks.purchaseSubscriptionWithBalance).not.toHaveBeenCalled()
      wrapper.unmount()
    }
  })

  // AC-031.3: balance subscription prices use USD with no provider fee or recharge promotion.
  it('supports balance-only checkout and displays USD without external rate, fee or recharge discounts', async () => {
    const wrapper = await mountPage()
    expect(wrapper.getComponent(PaymentMethodSelector).props('selected')).toBe('balance')
    expect(wrapper.get('[data-testid="balance-subscription-charge"]').text()).toBe('$10.00')
    expect(wrapper.get('[data-testid="balance-subscription-remaining"]').text()).toBe('$20.00')
    expect(wrapper.get('[data-testid="confirm-balance-subscription"]').text()).toContain('payment.balanceSubscription.confirm')
    expect(wrapper.get('[data-testid="confirm-balance-subscription"]').attributes('disabled')).toBeUndefined()
    expect(mocks.purchaseSubscriptionWithBalance).not.toHaveBeenCalled()
  })

  // AC-031.10: insufficient funds cannot submit even when rounded display amounts look equal.
  it('blocks an insufficient balance, including fractional cents, before making a request', async () => {
    mocks.setUser({ id: 9, username: 'demo', balance: 9.999 })
    const wrapper = await mountPage()
    expect(wrapper.get('[data-testid="confirm-balance-subscription"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="balance-subscription-insufficient"]').text()).toContain('payment.balanceSubscription.insufficient')
    await wrapper.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    expect(mocks.purchaseSubscriptionWithBalance).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="purchase-cdk"]').exists()).toBe(true)
  })

  // AC-031.5, AC-031.10: exact funds are spendable and a legitimate zero balance is success.
  it('allows an exact balance and accepts a completed purchase that leaves zero', async () => {
    mocks.setUser({ id: 9, username: 'demo', balance: 10 })
    mocks.purchaseSubscriptionWithBalance.mockResolvedValue(completed({ balance: 0 }))
    const wrapper = await mountPage()
    expect(wrapper.get('[data-testid="balance-subscription-remaining"]').text()).toBe('$0.00')
    expect(wrapper.get('[data-testid="confirm-balance-subscription"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="balance-subscription-success"]').text()).toContain('$0.00')
    expect(wrapper.get('[data-testid="balance-subscription-success"]').text()).toContain('payment.balanceSubscription.balanceAfterPayment')
    expect(mocks.purchaseSubscriptionWithBalance).toHaveBeenCalledTimes(1)
  })

  // AC-031.11: the existing external subscription payment payload remains intact.
  it('keeps external checkout available and never sends a recharge through the balance endpoint', async () => {
    const wrapper = await mountPage({ methods: { wxpay: { currency: 'CNY', daily_limit: 0, daily_used: 0, daily_remaining: 0, single_min: 0, single_max: 0, fee_rate: 0, available: true } } })
    expect(wrapper.getComponent(PaymentMethodSelector).props('selected')).toBe('wxpay')
    expect(wrapper.find('[data-testid="confirm-balance-subscription"]').exists()).toBe(false)
    mocks.createOrder.mockResolvedValue({ order_id: 1, amount: 10, pay_amount: 78.65, currency: 'CNY', fee_rate: 10, qr_code: 'wxpay-qr', expires_at: '2099-01-01' })
    await wrapper.findAll('button').find(button => button.text().startsWith('payment.createOrder'))!.trigger('click')
    await flushPromises()
    expect(mocks.createOrder).toHaveBeenCalledWith(expect.objectContaining({ order_type: 'subscription', payment_type: 'wxpay', amount: 10, plan_id: 7 }))
    expect(mocks.purchaseSubscriptionWithBalance).not.toHaveBeenCalled()
  })

  // AC-031.11: the top-up amount watcher must not override the subscription payment choice.
  it('keeps balance selected when entering subscriptions after typing a recharge amount', async () => {
    mocks.route.query = { tab: 'recharge' }
    const wrapper = await mountPage({ methods: { wxpay: { currency: 'CNY', daily_limit: 0, daily_used: 0, daily_remaining: 0, single_min: 0, single_max: 0, fee_rate: 0, available: true } } })
    wrapper.getComponent(AmountInput).vm.$emit('update:modelValue', 50)
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === 'payment.tabSubscribe')!.trigger('click')
    wrapper.getComponent(SubscriptionPlanCard).vm.$emit('select', checkout().plans[0])
    await flushPromises()
    selectBalance(wrapper)
    await flushPromises()
    expect(wrapper.getComponent(PaymentMethodSelector).props('selected')).toBe('balance')
    expect(wrapper.get('[data-testid="balance-subscription-charge"]').text()).toBe('$10.00')
  })

  // AC-031.10: a pending purchase locks all controls that could create a second intent.
  it('prevents repeat submissions and plan/method cancellation while awaiting the purchase', async () => {
    let resolve!: (value: ReturnType<typeof completed>) => void
    mocks.purchaseSubscriptionWithBalance.mockReturnValue(new Promise(r => { resolve = r }))
    const wrapper = await mountPage()
    const button = wrapper.get('[data-testid="confirm-balance-subscription"]')
    await button.trigger('click')
    await button.trigger('click')
    expect(mocks.purchaseSubscriptionWithBalance).toHaveBeenCalledTimes(1)
    expect(button.attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="cancel-subscription-plan"]').attributes('disabled')).toBeDefined()
    wrapper.getComponent(PaymentMethodSelector).vm.$emit('select', 'wxpay')
    await flushPromises()
    expect(wrapper.getComponent(PaymentMethodSelector).props('selected')).toBe('balance')
    resolve(completed())
    await flushPromises()
    expect(wrapper.find('[data-testid="balance-subscription-success"]').exists()).toBe(true)
  })

  it('reuses the UUID after an ambiguous network failure and succeeds without opening an external flow', async () => {
    const openWindow = vi.spyOn(window, 'open').mockReturnValue(null)
    mocks.purchaseSubscriptionWithBalance.mockRejectedValueOnce({ status: 0, message: 'network error' }).mockResolvedValueOnce(completed())
    const wrapper = await mountPage()
    await wrapper.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    const first = mocks.purchaseSubscriptionWithBalance.mock.calls[0][0]
    expect(first).toEqual({ plan_id: 7, expected_amount: 10, idempotency_key: expect.stringMatching(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i) })
    expect(wrapper.get('[data-testid="balance-subscription-error"]').text()).toContain('payment.balanceSubscription.unknown')
    await wrapper.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    expect(mocks.purchaseSubscriptionWithBalance.mock.calls[1][0]).toEqual(first)
    expect(wrapper.find('[data-testid="balance-subscription-success"]').exists()).toBe(true)
    expect(mocks.createOrder).not.toHaveBeenCalled()
    expect(openWindow).not.toHaveBeenCalled()
    expect(mocks.routerPush).not.toHaveBeenCalled()
    expect(mocks.refreshUser).toHaveBeenCalledTimes(1)
    expect(mocks.fetchActiveSubscriptions).toHaveBeenCalledWith(true)
  })

  it('retains the same unresolved purchase key after a page remount', async () => {
    mocks.purchaseSubscriptionWithBalance.mockRejectedValueOnce({ status: 0 }).mockResolvedValueOnce(completed())
    const firstPage = await mountPage()
    await firstPage.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    const first = mocks.purchaseSubscriptionWithBalance.mock.calls[0][0]
    firstPage.unmount()
    const secondPage = await mountPage()
    await secondPage.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    expect(mocks.purchaseSubscriptionWithBalance.mock.calls[1][0]).toEqual(first)
  })

  // AC-031.6, AC-031.10: a committed purchase can be replayed after its response is lost.
  it('checks a recovered purchase with zero balance and a newer plan price using the original quote', async () => {
    mocks.setUser({ id: 9, username: 'demo', balance: 10 })
    mocks.purchaseSubscriptionWithBalance.mockRejectedValueOnce({ status: 0 }).mockResolvedValueOnce(completed({ balance: 0 }))
    const firstPage = await mountPage()
    await firstPage.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    const first = mocks.purchaseSubscriptionWithBalance.mock.calls[0][0]
    firstPage.unmount()
    mocks.setUser({ id: 9, username: 'demo', balance: 0 })
    const secondPage = await mountPage({ plans: [{ ...checkout().plans[0], price: 20 }] })
    const verify = secondPage.get('[data-testid="confirm-balance-subscription"]')
    expect(verify.attributes('disabled')).toBeUndefined()
    expect(verify.text()).toContain('payment.balanceSubscription.verify')
    expect(secondPage.get('[data-testid="balance-subscription-charge"]').text()).toBe('$10.00')
    expect(secondPage.find('[data-testid="balance-subscription-insufficient"]').exists()).toBe(false)
    await verify.trigger('click')
    await flushPromises()
    expect(mocks.purchaseSubscriptionWithBalance.mock.calls[1][0]).toEqual(first)
    expect(secondPage.get('[data-testid="balance-subscription-success"]').text()).toContain('$0.00')
  })

  // AC-031.3, AC-031.10: changed prices require a new explicit confirmation, not an automatic retry.
  it('refreshes a changed plan price and waits for explicit confirmation of the new quote', async () => {
    mocks.purchaseSubscriptionWithBalance
      .mockRejectedValueOnce({ status: 409, reason: 'PLAN_PRICE_CHANGED' })
      .mockResolvedValueOnce(completed({ amount: 20, pay_amount: 20, balance: 10 }))
    const wrapper = await mountPage()
    const changedPlan = { ...checkout().plans[0], price: 20 }
    // checkout-info returns its sale catalog without the admin-only for_sale field.
    delete (changedPlan as Partial<typeof changedPlan>).for_sale
    mocks.getCheckoutInfo.mockResolvedValue({ data: checkout({ plans: [changedPlan] }) })
    await wrapper.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    const first = mocks.purchaseSubscriptionWithBalance.mock.calls[0][0]
    expect(first.expected_amount).toBe(10)
    expect(mocks.getCheckoutInfo).toHaveBeenCalledTimes(2)
    expect(mocks.purchaseSubscriptionWithBalance).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid="balance-subscription-charge"]').text()).toBe('$20.00')
    expect(wrapper.get('[data-testid="balance-subscription-error"]').text()).toContain('payment.balanceSubscription.priceChanged')
    const confirm = wrapper.get('[data-testid="confirm-balance-subscription"]')
    expect(confirm.text()).toContain('payment.balanceSubscription.confirm')
    expect(confirm.text()).not.toContain('payment.balanceSubscription.verify')
    await confirm.trigger('click')
    await flushPromises()
    expect(mocks.purchaseSubscriptionWithBalance.mock.calls[1][0]).toEqual({ ...first, expected_amount: 20 })
    expect(wrapper.find('[data-testid="balance-subscription-success"]').exists()).toBe(true)
  })

  it('blocks the stale quote when price refresh fails and allows confirmation only after a successful refresh', async () => {
    mocks.purchaseSubscriptionWithBalance.mockRejectedValueOnce({ status: 409, reason: 'PLAN_PRICE_CHANGED' }).mockResolvedValueOnce(completed())
    const wrapper = await mountPage()
    mocks.getCheckoutInfo.mockRejectedValueOnce({ status: 503 })
    await wrapper.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="confirm-balance-subscription"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="balance-subscription-error"]').text()).toContain('payment.balanceSubscription.priceRefreshFailed')
    await wrapper.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    expect(mocks.purchaseSubscriptionWithBalance).toHaveBeenCalledTimes(1)
    mocks.getCheckoutInfo.mockResolvedValue({ data: checkout({ plans: [{ ...checkout().plans[0], price: 12 }] }) })
    await wrapper.get('[data-testid="refresh-balance-subscription-price"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="balance-subscription-charge"]').text()).toBe('$12.00')
    expect(wrapper.get('[data-testid="confirm-balance-subscription"]').attributes('disabled')).toBeUndefined()
    expect(mocks.purchaseSubscriptionWithBalance).toHaveBeenCalledTimes(1)
  })

  it('creates a new intent after explicitly canceling and selecting a plan again', async () => {
    mocks.purchaseSubscriptionWithBalance.mockRejectedValueOnce({ status: 400, reason: 'BALANCE_INSUFFICIENT' }).mockResolvedValueOnce(completed())
    const wrapper = await mountPage()
    await wrapper.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    const first = mocks.purchaseSubscriptionWithBalance.mock.calls[0][0]
    await wrapper.get('[data-testid="cancel-subscription-plan"]').trigger('click')
    wrapper.getComponent(SubscriptionPlanCard).vm.$emit('select', checkout().plans[0])
    await flushPromises()
    selectBalance(wrapper)
    await flushPromises()
    await wrapper.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    expect(mocks.purchaseSubscriptionWithBalance.mock.calls[1][0].idempotency_key).not.toBe(first.idempotency_key)
  })

  // AC-031.10: an unknown payment cannot be discarded to create another charge.
  it('locks cancellation and method changes after an unknown result, including a generic 404', async () => {
    mocks.purchaseSubscriptionWithBalance.mockRejectedValueOnce({ status: 404, message: 'old server' }).mockResolvedValueOnce(completed())
    const wrapper = await mountPage()
    await wrapper.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    const first = mocks.purchaseSubscriptionWithBalance.mock.calls[0][0]
    const cancel = wrapper.get('[data-testid="cancel-subscription-plan"]')
    expect(cancel.attributes('disabled')).toBeDefined()
    await cancel.trigger('click')
    expect(wrapper.findComponent(SubscriptionPlanCard).exists()).toBe(false)
    wrapper.getComponent(PaymentMethodSelector).vm.$emit('select', 'wxpay')
    await flushPromises()
    expect(wrapper.getComponent(PaymentMethodSelector).props('selected')).toBe('balance')
    expect(wrapper.get('[data-testid="confirm-balance-subscription"]').text()).toContain('payment.balanceSubscription.verify')
    await wrapper.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    expect(mocks.purchaseSubscriptionWithBalance.mock.calls[1][0]).toEqual(first)
  })

  // AC-031.6, AC-031.10: recovery does not depend on the old plan remaining in the sale catalog.
  it('restores an unknown purchase on the ordinary purchase route when its plan is no longer for sale', async () => {
    mocks.purchaseSubscriptionWithBalance.mockRejectedValueOnce({ status: 0 }).mockResolvedValueOnce(completed())
    const firstPage = await mountPage()
    await firstPage.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    const first = mocks.purchaseSubscriptionWithBalance.mock.calls[0][0]
    firstPage.unmount()
    mocks.route.query = {}
    mocks.setUser({ id: 9, username: 'demo', balance: 0 })
    const recoveryPage = await mountPage({ plans: [] })
    const verify = recoveryPage.get('[data-testid="confirm-balance-subscription"]')
    expect(verify.attributes('disabled')).toBeUndefined()
    expect(verify.text()).toContain('payment.balanceSubscription.verify')
    await verify.trigger('click')
    await flushPromises()
    expect(mocks.purchaseSubscriptionWithBalance.mock.calls[1][0]).toEqual(first)
    expect(recoveryPage.find('[data-testid="balance-subscription-success"]').exists()).toBe(true)
  })

  // AC-031.11: fulfillment is authoritative even when subsequent account reads fail.
  it('preserves the completed purchase when both follow-up refreshes fail', async () => {
    mocks.purchaseSubscriptionWithBalance.mockResolvedValue(completed())
    mocks.refreshUser.mockRejectedValue(new Error('profile unavailable'))
    mocks.fetchActiveSubscriptions.mockResolvedValueOnce([]).mockRejectedValueOnce(new Error('subscriptions unavailable'))
    const wrapper = await mountPage()
    await wrapper.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="balance-subscription-success"]').text()).toContain('payment.result.subscriptionSuccess')
    expect(wrapper.get('[data-testid="balance-subscription-refresh-warning"]').text()).toContain('payment.balanceSubscription.refreshWarning')
    expect(wrapper.find('[data-testid="confirm-balance-subscription"]').exists()).toBe(false)
    expect(mocks.refreshUser).toHaveBeenCalledTimes(1)
    expect(mocks.fetchActiveSubscriptions).toHaveBeenCalledWith(true)
    expect(mocks.showError).not.toHaveBeenCalled()
    expect(mocks.purchaseSubscriptionWithBalance).toHaveBeenCalledTimes(1)
  })

  it('does not show success for a malformed or incomplete purchase response', async () => {
    mocks.purchaseSubscriptionWithBalance.mockResolvedValue(completed({ status: 'PAID' }))
    const wrapper = await mountPage()
    await wrapper.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="balance-subscription-success"]').exists()).toBe(false)
    expect(mocks.refreshUser).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="balance-subscription-error"]').text()).toContain('payment.balanceSubscription.unknown')
  })

  it('keeps a server-side insufficient-balance error distinct from an uncertain network result', async () => {
    mocks.purchaseSubscriptionWithBalance.mockRejectedValue({ status: 400, reason: 'BALANCE_INSUFFICIENT', message: 'Balance changed; top up before retrying.' })
    const wrapper = await mountPage()
    await wrapper.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="balance-subscription-error"]').text()).toContain('Balance changed; top up before retrying.')
    expect(wrapper.find('[data-testid="balance-subscription-success"]').exists()).toBe(false)
    expect(mocks.fetchActiveSubscriptions).not.toHaveBeenCalledWith(true)
    mocks.setUser({ id: 9, username: 'demo', balance: 0 })
    await flushPromises()
    expect(wrapper.get('[data-testid="confirm-balance-subscription"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="confirm-balance-subscription"]').text()).toContain('payment.balanceSubscription.confirm')
    expect(wrapper.get('[data-testid="confirm-balance-subscription"]').text()).not.toContain('payment.balanceSubscription.verify')
  })

  // AC-031.7: the browser recovery key is also scoped to the authenticated account.
  it('does not reuse another account\'s unresolved intent', async () => {
    mocks.purchaseSubscriptionWithBalance.mockRejectedValueOnce({ status: 0 }).mockResolvedValueOnce(completed())
    const firstPage = await mountPage()
    await firstPage.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    const first = mocks.purchaseSubscriptionWithBalance.mock.calls[0][0]
    firstPage.unmount()
    mocks.setUser({ id: 10, username: 'other', balance: 30 })
    const secondPage = await mountPage()
    await secondPage.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    expect(mocks.purchaseSubscriptionWithBalance.mock.calls[1][0].idempotency_key).not.toBe(first.idempotency_key)
  })

  // AC-031.7: another account's successful checkout must not erase the first account's pending intent.
  it('preserves the first account recovery when another account completes a purchase', async () => {
    mocks.purchaseSubscriptionWithBalance.mockRejectedValueOnce({ status: 0 }).mockResolvedValue(completed())
    const firstPage = await mountPage()
    await firstPage.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    const first = mocks.purchaseSubscriptionWithBalance.mock.calls[0][0]
    firstPage.unmount()
    mocks.setUser({ id: 10, username: 'other', balance: 30 })
    const otherPage = await mountPage()
    await otherPage.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    otherPage.unmount()
    mocks.setUser({ id: 9, username: 'demo', balance: 0 })
    const returnedPage = await mountPage()
    expect(returnedPage.get('[data-testid="confirm-balance-subscription"]').attributes('disabled')).toBeUndefined()
    await returnedPage.get('[data-testid="confirm-balance-subscription"]').trigger('click')
    await flushPromises()
    expect(mocks.purchaseSubscriptionWithBalance.mock.calls[2][0]).toEqual(first)
  })
})
