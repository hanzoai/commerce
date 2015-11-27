crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicList = require './list'

class OrderList extends BasicList
  tag: 'order-list'
  path: 'order'
  headers: [
    field('id', 'Number', 'id', 'id-display:number id-path:#order dontsort')
    field('currency', 'Currency', 'upper')
    field('total', 'Total', 'money')
    field('status', 'Order Status')
    field('paymentStatus', 'Paid')
    field('fulfillmentStatus', 'Fulfillment Status')
    field('couponCodes', 'Coupon(s)', 'id-list', 'id-path:#coupon')
    field('createdAt', 'Created', 'date')
    field('updatedAt', 'Last Updated', 'ago')
  ]

OrderList.register()
