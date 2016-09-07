_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

input = require '../input'
BasicFormView = require '../basic'
Form = require './form'

Api = crowdcontrol.data.Api

class SubscriberForm extends Form
  tag: 'subscriber-form'
  redirectPath: 'subscribers'
  path: 'subscriber'

  inputConfigs: [
    input('id', '', 'static'),
    input('email', 'Email')
    input('mailingListId', '', 'id id-path:#mailinglist')
    input('userId', '', 'static')
    input('unsubscribed', 'Unsubscribed', 'switch')
    input('metadata', '{}', 'text')

    input('client.ip', '', 'static')
    input('client.userAgent', '', 'static')
    input('client.language', '', 'static')
    input('client.referer', '', 'static')

    input('client.city', '', 'static')
    input('client.region', '', 'static')
    input('client.country', '', 'static')

    input('createdAt', '', 'static-date'),
    input('updatedAt', '', 'static-date'),
  ]

SubscriberForm.register()

class SubscriberUserStaticForm extends BasicFormView
  tag: 'subscriber-user-static-form'
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
    "#{Events.Form.Prefill}": (subscriberModel)->
      @loading = true

      @subscriberId = subscriberModel.id
      userId = subscriberModel?.userId
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

SubscriberUserStaticForm.register()

module.exports = SubscriberForm
