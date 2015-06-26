riot = require 'riot'
_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'
Api = crowdcontrol.data.Api
Source = crowdcontrol.data.Source

input = require '../input'
BasicFormView = require '../basic'

class UserFormView extends BasicFormView
  tag: 'user-form'
  path: 'user'
  html: require './template.html'

  # model that stores the last model queried
  resetModel: null

  inputConfigs:[
    input('id', '', 'static'),
    input('email', 'your@email.com', 'required email email-unique email-unique-exception:'),
    input('firstName', 'First Name', 'required'),
    input('lastName', 'Last Name', 'required'),
    input('phone', 'Phone'),
    input('createdAt', '', 'static-date'),
    input('updatedAt', '', 'static-date'),

    input('billingAddress.line1', 'Street Address'),
    input('billingAddress.line2', 'Apt/Suite Number'),
    input('billingAddress.city', 'City'),
    input('billingAddress.state', 'State'),
    input('billingAddress.postalCode', 'Postal/ZIP Code'),
    input('billingAddress.country', 'Choose a Country...', 'country-select'),

    input('shippingAddress.line1', 'Street Address'),
    input('shippingAddress.line2', 'Apt/Suite Number'),
    input('shippingAddress.city', 'City'),
    input('shippingAddress.state', 'State'),
    input('shippingAddress.postalCode', 'Postal/ZIP Code', 'postal-code'),
    input('shippingAddress.country', 'Choose a Country...', 'country-select'),
  ]

  mixins:
    reset: (event)->
      @model = _.deepExtend {}, @view.resetModel
      @view.initFormGroup.apply @
      riot.update()
      event.preventDefault()
      event.stopPropagation()

  submit: ()->
    p = super()
    p.then (data)=>
      @resetModel = _.deepExtend {}, data.data

  loadData: (model)->
    @inputConfigs[1].hints += model.email
    @resetModel = _.deepExtend {}, model

new UserFormView

module.exports = UserFormView
