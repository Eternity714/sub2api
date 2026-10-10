import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
  },
}))

import { paymentAPI } from '@/api/payment'

describe('payment api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    get.mockResolvedValue({ data: {} })
    post.mockResolvedValue({ data: {} })
  })

  it('keeps legacy public out_trade_no verification for upgrade compatibility', async () => {
    await paymentAPI.verifyOrderPublic('legacy-order-no')

    expect(post).toHaveBeenCalledWith('/payment/public/orders/verify', {
      out_trade_no: 'legacy-order-no',
    })
  })

  it('keeps signed public resume-token resolve endpoint', async () => {
    await paymentAPI.resolveOrderPublicByResumeToken('resume-token-123')

    expect(post).toHaveBeenCalledWith('/payment/public/orders/resolve', {
      resume_token: 'resume-token-123',
    })
  })

  // AC-031.3, AC-031.10: the confirmed quote guards against a changed server price.
  it('purchases a subscription with its plan, stable intent key and confirmed USD quote', async () => {
    await paymentAPI.purchaseSubscriptionWithBalance({
      plan_id: 7,
      idempotency_key: '11111111-1111-4111-8111-111111111111',
      expected_amount: 10,
    })

    expect(post).toHaveBeenCalledWith('/payment/orders/balance-subscription', {
      plan_id: 7,
      idempotency_key: '11111111-1111-4111-8111-111111111111',
      expected_amount: 10,
    })
  })
})
