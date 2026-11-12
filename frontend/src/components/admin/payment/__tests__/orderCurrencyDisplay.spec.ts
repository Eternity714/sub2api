import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { PaymentOrder } from '@/types/payment'
import AdminOrderDetail from '../AdminOrderDetail.vue'
import AdminOrderTable from '../AdminOrderTable.vue'
import AdminRefundDialog from '../AdminRefundDialog.vue'
import OrderTable from '@/components/payment/OrderTable.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const BaseDialogStub = {
  props: ['show'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
}

const DataTableStub = {
  props: ['data'],
  template: `
    <div>
      <div v-for="row in data" :key="row.id">
        <slot name="cell-pay_amount" :value="row.pay_amount" :row="row" />
        <slot name="cell-payment_type" :value="row.payment_type" :row="row" />
        <slot name="cell-actions" :row="row" />
      </div>
    </div>
  `,
}

function orderFactory(overrides: Partial<PaymentOrder> = {}): PaymentOrder {
  return {
    id: 1,
    user_id: 10,
    amount: 100,
    pay_amount: 108,
    currency: 'USD',
    fee_rate: 8,
    payment_type: 'stripe',
    out_trade_no: 'sub2_202606250001',
    status: 'COMPLETED',
    order_type: 'subscription',
    created_at: '2026-06-25T10:00:00Z',
    expires_at: '2026-06-25T10:30:00Z',
    refund_amount: 25,
    ...overrides,
  }
}

describe('admin order currency display', () => {
  // AC-031.9: balance orders do not offer provider refunds in either admin entry.
  it('hides refunds for balance subscription orders while retaining external refunds', async () => {
    const balanceOrder = orderFactory({ payment_type: 'balance', currency: 'USD', refund_amount: 0 })
    const table = mount(AdminOrderTable, {
      props: { orders: [balanceOrder], loading: false, page: 1, pageSize: 20, total: 1 },
      global: { stubs: { DataTable: DataTableStub, Select: true, SearchInput: true, Icon: true } },
    })
    const detail = mount(AdminOrderDetail, {
      props: { show: true, order: balanceOrder },
      global: { stubs: { BaseDialog: BaseDialogStub } },
    })
    expect(table.text()).toContain('payment.methods.balance')
    expect(table.findAll('button').some(button => button.text() === 'payment.admin.refund')).toBe(false)
    expect(detail.findAll('button').some(button => button.text() === 'payment.admin.refund')).toBe(false)
    await table.setProps({ orders: [orderFactory()] })
    await detail.setProps({ order: orderFactory() })
    expect(table.findAll('button').some(button => button.text() === 'payment.admin.refund')).toBe(true)
    expect(detail.findAll('button').some(button => button.text() === 'payment.admin.refund')).toBe(true)
  })

  it('uses order currency for paid/base/fee amounts and USD for credited/refund amounts', () => {
    const wrapper = mount(AdminOrderDetail, {
      props: {
        show: true,
        order: orderFactory({ currency: 'CNY' }),
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
        },
      },
    })

    const text = wrapper.text()
    expect(text).toContain('¥100.00')
    expect(text).toContain('¥8.00')
    expect(text).toContain('¥108.00')
    expect(text).toContain('$100.00')
    expect(text).toContain('$25.00')
  })

  it('uses order currency for pay_amount and USD for refundable balance amounts', () => {
    const wrapper = mount(AdminRefundDialog, {
      props: {
        show: true,
        order: orderFactory({
          currency: 'USD',
          status: 'PARTIALLY_REFUNDED',
          refund_amount: 20,
        }),
        userBalance: 200,
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
        },
      },
    })

    const text = wrapper.text()
    expect(text).toContain('$108.00')
    expect(text).toContain('$100.00')
    expect(text).toContain('$20.00')
    expect(text).toContain('$80.00')
    expect(text).toContain('$200.00')
  })

  it('renders payment currency consistently in the shared order table', () => {
    const wrapper = mount(OrderTable, {
      props: {
        orders: [
          orderFactory({ id: 1, currency: 'USD', amount: 100, pay_amount: 108 }),
          orderFactory({ id: 2, currency: 'CNY', amount: 100, pay_amount: 108 }),
        ],
        loading: false,
        showUser: true,
      },
      global: {
        stubs: {
          DataTable: DataTableStub,
          OrderStatusBadge: true,
        },
      },
    })

    const text = wrapper.text()
    expect(text).toContain('$108.00')
    expect(text).toContain('¥108.00')
    expect(text).toContain('$100.00')
  })

  it('renders payment currency consistently in the admin order table', () => {
    const wrapper = mount(AdminOrderTable, {
      props: {
        orders: [
          orderFactory({ id: 1, currency: 'USD', amount: 100, pay_amount: 108 }),
          orderFactory({ id: 2, currency: 'CNY', amount: 100, pay_amount: 108 }),
        ],
        loading: false,
        page: 1,
        pageSize: 20,
        total: 2,
      },
      global: {
        stubs: {
          DataTable: DataTableStub,
          Icon: true,
          Pagination: true,
          Select: true,
        },
      },
    })

    const text = wrapper.text()
    expect(text).toContain('$108.00')
    expect(text).toContain('¥108.00')
    expect(text).toContain('$100.00')
  })
})
