_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

input = require '../input'
BasicFormView = require '../basic'
Form = require './form'

class UserForm extends Form
  tag: 'user-form'
  redirectPath: 'users'
  path: 'user'

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

  loadData: (model)->
    super
    @inputConfigs[1].hints['email-unique-exception'] = model.email

UserForm.register()

module.exports = UserForm
