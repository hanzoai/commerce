crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicPagedTable = require './paged'

class PaymentPagedTable extends BasicPagedTable
  tag: 'payment-paged-table'
  path: 'payment'
  headers: [
    field('id', 'Id', 'id', 'id-path:#payment')
    field('account.chargeId', 'Stripe', 'id', 'id-path://dashboard.stripe.com/payments')
    field('orderId', 'Order', 'id', 'id-path:#order')
    field('currency', 'Currency', 'upper')
    field('amount', 'Total', 'money')
    # field('client.referer', 'Referrer')
    field('status', 'Status')
    field('createdAt', 'Created', 'date')
    field('updatedAt', 'Last Updated', 'ago')
  ]

PaymentPagedTable.register()

