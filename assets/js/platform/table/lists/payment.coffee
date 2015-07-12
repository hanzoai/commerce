crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicList = require './list'

class PaymentList extends BasicList
  tag: 'payment-list'
  path: 'payment'
  headers: [
    field('account.chargeId', 'Id', 'id', 'id-path://dashboard.stripe.com/payments')
    field('currency', 'Currency', 'upper')
    field('amount', 'Total', 'money')
    field('status', 'Status')
    field('createdAt', 'Created On', 'date')
    # field('updatedAt', 'Last Updated', 'ago')
  ]

PaymentList.register()

