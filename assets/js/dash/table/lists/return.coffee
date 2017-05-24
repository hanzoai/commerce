crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicList = require './list'

class ReturnList extends BasicList
  tag: 'return-list'
  path: 'return'
  headers: [
    field('id', 'Id', '', '')
    field('externalId', 'Shipwire', 'id', 'id-path://merchant.shipwire.com/merchants/store/tracking/orderId/')
    field('createdAt', 'Created', 'date')
    # field('updatedAt', 'Last Updated', 'ago')
  ]

ReturnList.register()

