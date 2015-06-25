riot = require 'riot'

crowdcontrol = require 'crowdcontrol'
Api = crowdcontrol.data.Api
Source = crowdcontrol.data.Source

input = require '../input'
BasicFormView = require '../basic'

class UserFormView extends BasicFormView
  tag: 'user-form'
  path: 'user'
  html: require './template.html'
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
    input('shippingAddress.postalCode', 'Postal/ZIP Code'),
    input('shippingAddress.country', 'Choose a Country...', 'country-select'),
  ]

  loadData: (model)->
    @inputConfigs[1].hints += model.email

new UserFormView

module.exports = UserFormView
