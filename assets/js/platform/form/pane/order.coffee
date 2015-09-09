_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

input = require '../input'
Form = require './pane'

class OrderFilterForm extends Pane
  tag: 'order-filter-pane'
  path: ''

  inputConfigs: [
    input('min-date', '', 'text')
    input('max-date', '', 'text')
  ]

  js: ()->
    super

OrderFilterForm.register()

module.exports = OrderFilterForm

