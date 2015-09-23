_ = require 'underscore'
moment = require 'moment'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

input = require '../input'
Pane = require './pane'

#TODO: Add actual localization stuff to util
localizeDate = (date)->
  tokens = date.split '/'
  return moment((tokens[2] ? '2015') + ' ' + (tokens[0] ? '01') + ' ' + (tokens[1] ? '01'), 'YYYY-MM-DD').format 'YYYY-MM-DD'

class OrderFilterPane extends Pane
  tag: 'order-filter-pane'
  html: require '../../templates/backend/form/pane/order.html'
  path: 'search/order'

  inputConfigs: [
    input('minTotal', '', 'money')
    input('maxTotal', '', 'money')
    input('currency', '', 'currency-type-select')
    input('minDate', '', 'date-picker')
    input('maxDate', '', 'date-picker')
    input('country', '', 'country-select')
    input('status', '', 'basic-select')
    input('paymentStatus', '', 'basic-select')
    input('fulfillmentStatus', '', 'basic-select')
    input('preorder', '', 'basic-select')
    input('confirmed', '', 'basic-select')
  ]

  statusOptions:
    _any:       'Any Order State'
    cancelled:  'Cancelled'
    completed:  'Completed'
    locked:     'Locked'
    open:       'Open'
    'on-hold':  'On Hold'

  paymentStatusOptions:
    _any:       'Any Payment State'
    credit:     'Credit'
    disputed:   'Disputed'
    failed:     'Failed'
    fraudulent: 'Fraud'
    paid:       'Paid'
    refunded:   'Refunded'
    unpaid:     'Unpaid'

  fulfillmentStatusOptions:
    _any:           'Any Fulfillment State'
    cancelled:      'Cancelled'
    processing:     'Processing'
    shipped:        'Shipped'
    unfulfilled:    'Unfulfilled'

  preorderOptions:
    _any:   'Any Preorder State'
    true:   'Preorder'
    false:  'Not Preorder'

  confirmedOptions:
    _any:   'Any Confirmation State'
    true:   'Confirmed'
    false:  'Unconfirmed'

  js: ()->
    @model =
      minTotal:           0
      maxTotal:           0
      country:            '_any'
      currency:           '_any'
      minDate:            '01/01/2015'
      maxDate:            moment().format 'L'
      status:             '_any'
      paymentStatus:      '_any'
      fulfillmentStatus:  '_any'
      preorder:           '_any'
      confirmed:          '_any'

    super

  queryString: ()->
    minDate = localizeDate(@model.minDate)
    maxDate = localizeDate(@model.maxDate)

    if moment(minDate, 'YYYY-MM-DD').isAfter moment(maxDate, 'YYYY-MM-DD')
      swap  = maxDate
      maxDate = minDate
      minDate = swap2

    @model.minDate = minDate
    @model.maxDate = maxDate

    riot.update()

    minDateStr = moment(minDate, 'YYYY-MM-DD').format 'YYYY-MM-DD'
    maxDateStr = moment(maxDate, 'YYYY-MM-DD').format 'YYYY-MM-DD'

    query = "CreatedAt >= #{minDateStr} AND CreatedAt <= #{maxDateStr}"
    if @model.country != '_any'
      query += " AND ShippingAddressCountryCode = \"#{ @model.country }\""

    if @model.currency != '_any'
      query += " AND Currency = \"#{ @model.currency }\""

    if @model.maxTotal != 0
      query += " AND Total >= #{ @model.minTotal } AND Total <= #{ @model.maxTotal }"

    if @model.status != '_any'
      query += " AND Status = \"#{ @model.status }\""

    if @model.paymentStatus != '_any'
      query += " AND PaymentStatus = \"#{ @model.paymentStatus }\""

    if @model.fulfillmentStatus != '_any'
      query += " AND FulfillmentStatus = \"#{ @model.fulfillmentStatus }\""

    if @model.preorder != '_any'
      query += " AND Preorder = \"#{ @model.preorder }\""

    if @model.confirmed != '_any'
      query += " AND Confirmed = \"#{ @model.confirmed }\""

    return query

OrderFilterPane.register()

module.exports = OrderFilterPane
