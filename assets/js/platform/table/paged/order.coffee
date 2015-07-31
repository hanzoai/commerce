crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicPagedTable = require './paged'

class OrderPagedTable extends BasicPagedTable
  tag: 'order-paged-table'
  path: 'order'
  headers: [
    field('id', 'Number', 'id', 'id-display:number id-path:#order')
    field('userId', 'User ID', 'id', 'id-path:#user')
    field('currency', 'Currency', 'upper')
    field('total', 'Total', 'money')
    field('status', 'Status')
    field('paymentStatus', 'Paid')
    field('fulfillmentStatus', 'Fullfilled')
    field('createdAt', 'Created On', 'date')
    field('updatedAt', 'Last Updated', 'ago')
  ]

OrderPagedTable.register()
