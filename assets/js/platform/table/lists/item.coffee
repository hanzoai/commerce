crowdcontrol = require 'crowdcontrol'

BasicFormView = require '../../form/basic'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicList = require './list'

class ItemList extends BasicList
  tag: 'item-list'
  path: 'item'
  headers: [
    field('productId', 'Product Slug', 'id', 'id-display:productSlug id-path:../product')
    field('productName', 'Product ')
    field('quantity', 'Quantity')
    # field('updatedAt', 'Last Updated', 'ago')
  ]

  events:
    "#{ BasicFormView.Events.Load }": (orderModel) ->
      @model = orderModel.items
      @update()

ItemList.register()

