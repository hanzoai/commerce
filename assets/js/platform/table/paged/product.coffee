crowdcontrol = require 'crowdcontrol'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicPagedTable = require './paged'

class ProductPagedTable extends BasicPagedTable
  tag: 'product-paged-table'
  path: 'product'
  headers: [
    field('id', 'Slug', 'id', 'id-display:slug id-path:#product')
    field('name', 'Name', 'upper')
    field('currency', 'Currency', 'upper')
    field('listPrice', 'List Price', 'money')
    field('price', 'Price', 'money')
    field('available', 'Available')
    field('createdAt', 'Created On', 'date')
    field('updatedAt', 'Last Updated', 'ago')
  ]

ProductPagedTable.register()
