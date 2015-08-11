crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicList = require './list'

class PaymentList extends BasicList
  tag: 'payment-list'
  path: 'payment'
  headers: [
    field('id', 'Id', 'id', 'id-path:#payment')
    field('account.chargeId', 'Stripe', 'id', 'id-path://dashboard.stripe.com/payments')
    field('currency', 'Currency', 'upper')
    field('amount', 'Total', 'money')
    field('status', 'Status')
    field('createdAt', 'Created', 'date')
    # field('updatedAt', 'Last Updated', 'ago')
  ]

PaymentList.register()

