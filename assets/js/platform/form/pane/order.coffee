_ = require 'underscore'
moment = require 'moment'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

input = require '../input'
Pane = require './pane'

class OrderFilterPane extends Pane
  tag: 'orders-filter-pane'
  html: require '../../templates/backend/form/pane/order.html'
  path: 'search/order'

  inputConfigs: [
    input('minDate', '', 'date-picker')
    input('maxDate', '', 'date-picker')
  ]

  model:
    minDate: '01/01/2015'
    maxDate:  moment().format 'L'

  queryString: ()->
    if moment(@model.minDate).isAfter(@model.maxDate)
      swap = @model.maxDate
      @modal.maxDate = @model.minDate
      @model.minDate = swap
      @update()

    minDate = moment(@model.minDate).format 'YYYY-M-D'
    maxDate = moment(@model.maxDate).format 'YYYY-M-D'

    return "CreatedAt >= #{minDate} AND CreatedAt <= #{maxDate}"

OrderFilterPane.register()

module.exports = OrderFilterPane
