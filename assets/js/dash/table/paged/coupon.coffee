crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicPagedTable = require './paged'

class CouponPagedTable extends BasicPagedTable
  tag: 'coupon-paged-table'
  path: 'coupon'
  headers: [
    field('id', 'Code', 'id', 'id-display:code id-path:#coupon')
    field('name', 'Name')
    field('type', 'Type')
    field('amount', 'Amount', 'money')
    field('enabled', 'Enabled')
    field('createdAt', 'Created On', 'date')
    field('updatedAt', 'Last Updated', 'ago')
  ]

CouponPagedTable.register()
