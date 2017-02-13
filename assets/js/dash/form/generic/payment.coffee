_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

input = require '../input'
BasicFormView = require '../basic'
Form = require './form'

Api = crowdcontrol.data.Api

class PaymentForm extends Form
  tag: 'payment-form'
  redirectPath: 'payments'
  path: 'payment'

  inputConfigs: [
    input('id', '', 'static'),
    input('type', '', 'static')
    input('orderId', '', 'id id-path:#order')
    input('amount', '', 'static-money')
    input('amountRefunded', '', 'static-money')
    input('fee', '', 'static-money')
    input('status', '', 'static')

    input('captured', '', 'static')
    input('live', '', 'static')
    input('test', '', 'static')

    input('client.ip', '', 'static')
    input('client.userAgent', '', 'static')
    input('client.language', '', 'static')
    input('client.referer', '', 'static')

    input('client.city', '', 'static')
    input('client.region', '', 'static')
    input('client.country', '', 'static')

    input('account.chargeId', '', 'id id-path://dashboard.stripe.com/charges')
    input('account.customerId', '', 'id id-path://dashboard.stripe.com/customers')
    input('account.cardId', '', 'static')
    input('account.balanceTransactionId', '', 'static')
    input('account.fingerprint', '', 'static')
    input('account.funding', '', 'static')
    input('account.lastFour', '', 'static')
    input('account.brand', '', 'static')
    input('account.month', '', 'static')
    input('account.year', '', 'static')
    input('account.country', '', 'static')

    input('createdAt', '', 'static-date'),
    input('updatedAt', '', 'static-date'),
  ]

PaymentForm.register()

class PaymentUserStaticForm extends BasicFormView
  tag: 'payment-user-static-form'
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
    "#{Events.Form.Prefill}": (paymentModel)->
      @loading = true

      @paymentId = paymentModel.id
      userId = paymentModel?.buyer?.userId
      if userId
        @path = @basePath + '/' + userId

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

PaymentUserStaticForm.register()

module.exports = PaymentForm
