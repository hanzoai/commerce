crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

BasicFormView = require '../../form/basic'

table = require '../types'
field = table.field

Source = crowdcontrol.data.Source
BasicList = require './list'

class ItemList extends BasicList
  tag: 'item-list'
  path: 'item'
  itemsModel: 'items'
  # html: require '../../templates/backend/table/lists/item.html'
  headers: [
    field('productId', 'Product Slug', 'id', 'id-display:productSlug id-path:../product')
    field('productName', 'Product ')
    field('quantity', 'Quantity')
    field('price', 'Unit Price', 'money')
    field('totalPrice', 'Total', 'total')
    # field('updatedAt', 'Last Updated', 'ago')
  ]

  js: (opts)->
    super
    @itemsModel = opts.itemsmodel ? @itemsModel

  events:
    "#{ Events.Form.Prefill }": (orderModel) ->
      @model = orderModel[@itemsModel]
      if @model?
        for item in @model
          item.currency = orderModel.currency
        @update()

ItemList.register()
