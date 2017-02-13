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
  html: require '../../templates/dash/form/pane/order.html'
  path: 'search/order'

  inputConfigs: [
    input('minTotal', '', 'money')
    input('maxTotal', '', 'money')
    input('currency', '', 'currency-type-select')
    input('minDate', '', 'date-picker')
    input('maxDate', '', 'date-picker')
    input('country', '', 'country-select')
    input('type', '', 'basic-select')
    input('couponCodes', 'Coupon Code')
    input('status', '', 'basic-select')
    input('paymentStatus', '', 'basic-select')
    input('paymentStatus', '', 'basic-select')
    input('fulfillmentStatus', '', 'basic-select')
    input('preorder', '', 'basic-select')
    input('confirmed', '', 'basic-select')
    input('productIds', '', 'product-type-select')
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

  typeOptions:
    _any:   'Any Payment Processor'
    stripe: 'Stripe'
    paypal: 'Paypal'

  js: ()->
    @model =
      maxDate:            moment().format 'L'
      minTotal:           0
      maxTotal:           0
      country:            '_any'
      currency:           '_any'
      minDate:            '01/01/2015'
      couponCodes:        ''
      type:               '_any'
      status:             '_any'
      paymentStatus:      '_any'
      fulfillmentStatus:  '_any'
      preorder:           '_any'
      confirmed:          '_any'
      productIds:         '_any'

    super

  queryString: ()->
    minDate = localizeDate(@model.minDate)
    maxDate = localizeDate(@model.maxDate)

    if moment(minDate, 'YYYY-MM-DD').isAfter moment(maxDate, 'YYYY-MM-DD')
      swap  = maxDate
      maxDate = minDate
      minDate = swap2

    riot.update()

    minDateStr = moment(minDate, 'YYYY-MM-DD').format 'YYYY-MM-DD'
    maxDateStr = moment(maxDate, 'YYYY-MM-DD').format 'YYYY-MM-DD'

    query = "CreatedAt >= #{encodeURIComponent minDateStr} AND CreatedAt <= #{encodeURIComponent maxDateStr}"
    if @model.country != '_any'
      query += " AND ShippingAddressCountryCode = \"#{ encodeURIComponent @model.country }\""

    if @model.currency != '_any'
      query += " AND Currency = \"#{ encodeURIComponent @model.currency }\""

    query += " AND Total >= #{ encodeURIComponent @model.minTotal }" if @model.minTotal != 0
    query += " AND Total <= #{ encodeURIComponent @model.maxTotal }" if @model.maxTotal != 0

    if @model.status != '_any'
      query += " AND Status = \"#{ encodeURIComponent @model.status }\""

    if @model.paymentStatus != '_any'
      query += " AND PaymentStatus = \"#{ encodeURIComponent @model.paymentStatus }\""

    if @model.fulfillmentStatus != '_any'
      query += " AND FulfillmentStatus = \"#{ encodeURIComponent @model.fulfillmentStatus }\""

    if @model.preorder != '_any'
      query += " AND Preorder = \"#{ encodeURIComponent @model.preorder }\""

    if @model.confirmed != '_any'
      query += " AND Confirmed = \"#{ encodeURIComponent @model.confirmed }\""

    if @model.type != '_any'
      query += " AND Type = \"#{ encodeURIComponent @model.type }\""

    if @model.couponCodes
      query += " AND CouponCodes = \"#{ encodeURIComponent @model.couponCodes }\""

    if @model.productIds != '_any'
      query += " AND ProductIds = \"#{ encodeURIComponent @model.productIds }\""

    return query

OrderFilterPane.register()

module.exports = OrderFilterPane
