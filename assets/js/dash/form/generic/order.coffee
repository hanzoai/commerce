_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

input = require '../input'
BasicFormView = require '../basic'
Form = require './form'

Api = crowdcontrol.data.Api

class OrderForm extends Form
  tag: 'order-form'
  redirectPath: 'orders'
  path: 'order'

  inputConfigs:[
    input('id', '', 'static'),
    input('number', '', 'static'),
    input('userId', '', 'id id-path:#user'),
    input('createdAt', '', 'static-date'),
    input('updatedAt', '', 'static-date'),

    input('shippingAddress.name', 'Recipient Name', 'required'),
    input('shippingAddress.line1', 'Street Address', 'required'),
    input('shippingAddress.line2', 'Apt/Suite Number'),
    input('shippingAddress.city', 'City', 'required'),
    input('shippingAddress.state', 'State', 'required'),
    input('shippingAddress.postalCode', 'Postal/ZIP Code', 'postal-code'),
    input('shippingAddress.country', 'Choose a Country...', 'country-select', 'required'),

    input('refundAmount', 'Refund Amount', 'money gtzero'),

    input('giftEmail', ''),
    input('giftMessage', ''),

    input('currency', '', 'static'),
    input('lineTotal', '', 'static-money'),
    input('discount', '', 'static-money'),
    input('subtotal', '', 'static-money'),
    input('shipping', '', 'static-money'),
    input('refunded', '', 'static-money'),
    input('tax', '', 'static-money'),
    input('total', '', 'static-money'),
    input('couponCodes', '', 'id-list id-path:#coupon')

    input('status', '', 'order-status-select'),
    input('paymentStatus', '', 'payment-status-select'),
    input('fulfillmentStatus', '', 'fulfillment-status-select'),

    input('shippingService', '', 'shipping-service-select'),
    input('sendReturnEmail', '', 'switch')
    input('payForReturn', '', 'switch')
    input('returns0', '', 'numeric')
    input('returns1', '', 'numeric')
    input('returns2', '', 'numeric')
    input('returns3', '', 'numeric')
    input('returns4', '', 'numeric')
    input('returnSummary', '', 'basic-textarea')
    input('fulfillment.externalId', '', 'static')
    input('fulfillment.carrier', '', 'static')
    input('fulfillment.service', '', 'static')
    input('fulfillment.pricing', '', 'static-money')
    input('fulfillment.pricingEstimate', '', 'static-money')

    input('metadata', '', 'static-pre'),
  ]

  events:
    "#{Events.Form.Prefill}": (orderModel)->
      orderModel.shippingService = @inputs.shippingService.model.value = 'GD'
      orderModel.sendReturnEmail = true
      orderModel.payForReturn = false
      for i in [0..5]
        if orderModel.items[i]?
          orderModel['returns' + i] = orderModel.items[i].quantity
          @inputs['returns' + i].model.value = orderModel.items[i].quantity
      riot.update()

  # hack for couponCodes because crowdcontrol doenst treat arrays as leaves
  initFormGroup: ()->
    super

    @inputs.sendReturnEmail.model.value = true
    @inputs.payForReturn.model.value = false
    @inputs.couponCodes.model.value = @model.couponCodes
    @inputs.refundAmount.model.value = @model.refundAmount = @model.total - @model.refunded

  refundModal: ()->

    value = $('#refundAmount').val()

    bootbox.dialog
      title: 'Are You Sure?'
      message: 'This will issue a ' + value + ' refund.'

      buttons:
        Refund:
          className: 'btn btn-danger'
          callback: ()=>
            @refund()

        "Don't Refund":
          className: 'btn btn-primary'
          callback: ()->
    return false

  shippingModal: ()->
    bootbox.dialog
      title: 'Are You Sure?'
      message: 'This will ship this order.'

      buttons:
        Ship:
          className: 'btn btn-danger'
          callback: ()=>
            @ship()

        "Don't Ship":
          className: 'btn btn-primary'
          callback: ()->
    return false

  returnModal: ()->
    bootbox.dialog
      title: 'Are You Sure?'
      message: 'This will start the return process forthis order.'

      buttons:
        Return:
          className: 'btn btn-danger'
          callback: ()=>
            @return()

        "Don't Return":
          className: 'btn btn-primary'
          callback: ()->
    return false

  refund: ()->
    @api.post(@path + '/refund',
      amount: @model.refundAmount
    ).finally (e)->
      console.log(e.stack) if e
      window.location.reload()

  ship: ()->
    @api.post('shipwire/order/' + @model.id,
      service: @model.shippingService
    ).finally (e)->
      console.log(e.stack) if e
      window.location.reload()

  return: ()->
    rtn =
      email: @model.sendReturnEmail
      prepaid: @model.payForReturn
      summary: @model.returnSummary
      items: []

    for i in [0..5]
      if @model['returnProductId'+i]?
        rtn.items.push
          quantity: @model['returns'+i]
          productId: @model['returnProductId'+i]

    @api.post('shipwire/return/' + @model.id, rtn).finally (e)->
      console.log(e.stack) if e
      window.location.reload()

OrderForm.register()

class OrderUserStaticForm extends BasicFormView
  tag: 'order-user-static-form'
  basePath: 'user'

  inputConfigs:[
    input('id', '', 'id id-path:#user'),
    input('email', 'your@email.com', 'static')
    input('firstName', 'First Name', 'static'),
    input('lastName', 'Last Name', 'static'),
    input('phone', 'Phone', 'static'),
    input('createdAt', '', 'static-date'),
    input('updatedAt', '', 'static-date'),
  ]

  events:
    "#{Events.Form.Prefill}": (orderModel)->
      @loading = true

      @orderId = orderModel.id

      if orderModel.userId
        @path = @basePath + '/' + orderModel.userId
        @api = api = Api.get('crowdstart')
        api.get(@path).then((res)=>

          if res.status != 200
            throw new Error("Form failed to load")

          @model = res.responseText
          @loadData @model

          @initFormGroup()
          riot.update()
        ).catch (e)=>
          @error = e
          console.log e.stack
          riot.update()
      else
        @error = new Error('No UserId')
        riot.update()

  js: ()->
    @initFormGroup()

  refund: (event)->
    if orderModel.userId
      @path = @basePath + '/' + orderModel.userId
      @api = api = Api.get('crowdstart')
      api.get(@path).then((res)->
        if res.status != 200
          throw new Error("Refund Failed")

      ).catch (e)=>
        @error = e
        console.log e.stack
        riot.update()

  resendOrderConfirmation: (event)->
    api = Api.get 'dash'

    api.get('sendorderconfirmation/' + @orderId)
    $(event.target).html 'Sent!'
    @sending = true

  resendShippingConfirmation: (event)->
    api = Api.get 'dash'

    api.get('sendfulfillmentconfirmation/' + @orderId)
    $(event.target).html 'Sent!'
    @sending = true

  resendRefundConfirmation: (event)->
    api = Api.get 'dash'

    api.get('sendrefundconfirmation/' + @orderId)
    $(event.target).html 'Sent!'
    @sending = true

OrderUserStaticForm.register()

module.exports = OrderForm
