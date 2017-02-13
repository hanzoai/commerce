crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicPagedTable = require './paged'

class StorePagedTable extends BasicPagedTable
  tag: 'store-paged-table'
  path: 'store'
  headers: [
    field('id', 'Slug', 'id', 'id-display:slug id-path:#store')
    field('name', 'Name')
    field('currency', 'Currency', 'upper')
    field('createdAt', 'Created On', 'date')
    field('updatedAt', 'Last Updated', 'ago')
  ]

StorePagedTable.register()
