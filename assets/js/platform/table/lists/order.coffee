crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicList = require './list'

m = crowdcontrol.utils.mediator

class OrderList extends BasicList
  tag: 'order-list'
  path: 'order'
  headers: [
    field('id', 'Id', 'id', 'id-path:../order/')
    field('currency', 'Currency', 'upper')
    field('total', 'Total', 'money')
    field('status', 'Order Status')
    field('fulfillmentStatus', 'Fulfillment Status')
    field('createdAt', 'Created On', 'date')
    # field('updatedAt', 'Last Updated', 'ago')
  ]

OrderList.register()
