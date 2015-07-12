_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'
m = crowdcontrol.utils.mediator

input = require './input'
BasicFormView = require './basic'
Api = crowdcontrol.data.Api

class OrderForm extends BasicFormView
  tag: 'order-form'
  redirectPath: 'orders'
  path: 'order'

  # model that stores the last model queried
  resetModel: null

  inputConfigs:[
    input('id', '', 'static'),
    input('number', '', 'static'),
    input('userId', '', 'id id-path:../user'),
    input('createdAt', '', 'static-date'),
    input('updatedAt', '', 'static-date'),

    input('shippingAddress.line1', 'Street Address'),
    input('shippingAddress.line2', 'Apt/Suite Number'),
    input('shippingAddress.city', 'City'),
    input('shippingAddress.state', 'State'),
    input('shippingAddress.postalCode', 'Postal/ZIP Code', 'postal-code'),
    input('shippingAddress.country', 'Choose a Country...', 'country-select'),

    input('currency', '', 'static'),
    input('lineTotal', '', 'static-money'),
    input('discount', '', 'static-money'),
    input('subtotal', '', 'static-money'),
    input('shipping', '', 'static-money'),
    input('tax', '', 'static-money'),
    input('total', '', 'static-money'),

    input('status', '', 'static'),
    input('paymentStatus', '', 'static'),
    input('fulfillmentStatus', '', 'static'),
  ]

  js: (opts)->
    #case sensitivity issues
    @orderId = opts.orderId = opts.orderId || opts.orderid

    @path += '/' + opts.orderId

    super

  reset: (event)->
    if event?
      event.preventDefault()

    @model = _.deepExtend {}, @resetModel
    @initFormGroup.apply @
    @_reset(event)
    riot.update()

  _reset: (event)->

  _submit: (event)->
    p = super
    p.then ()=>
      @resetModel = _.deepExtend {}, @model

  loadData: (model)->
    @resetModel = _.deepExtend {}, @model

OrderForm.register()

class OrderUserStaticForm extends BasicFormView
  tag: 'order-user-static-form'
  basePath: 'user'

  inputConfigs:[
    input('id', '', 'id id-path:../user'),
    input('email', 'your@email.com', 'static')
    input('firstName', 'First Name', 'static'),
    input('lastName', 'Last Name', 'static'),
    input('phone', 'Phone', 'static'),
    input('createdAt', '', 'static-date'),
    input('updatedAt', '', 'static-date'),
  ]

  events:
    "#{BasicFormView.Events.Load}": (orderModel)->
      @loading = true
      m.trigger 'start-spin', @path + '-form-load'

      @path = @basePath + '/' + orderModel.userId

      @api = api = Api.get('crowdstart')
      api.get(@path).then (res)=>
        m.trigger 'stop-spin', @path + '-form-load'

        if res.status != 200
          throw new Error("Form failed to load")

        @model = res.responseText
        @loadData @model

        @initFormGroup()
        riot.update()

  js: ()->
    @initFormGroup()

OrderUserStaticForm.register()

module.exports = OrderForm
