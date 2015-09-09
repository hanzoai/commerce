_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

input = require '../input'
Pane = require './pane'

class OrderFilterPane extends Pane
  tag: 'orders-filter-pane'
  html: require '../../templates/backend/form/pane/order.html'
  path: ''

  inputConfigs: [
    input('min-date', '', 'text')
    input('max-date', '', 'text')
  ]

  js: ()->
    super

OrderFilterPane.register()

module.exports = OrderFilterPane

