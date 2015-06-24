riot = require 'riot'

crowdcontrol = require 'crowdcontrol'
Api = crowdcontrol.data.Api
Source = crowdcontrol.data.Source

FormView = crowdcontrol.view.form.FormView

input = require '../input'

class UserFormView extends FormView
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
    input('billingAddress.country', 'Choose a Country...', 'country'),

    input('shippingAddress.line1', 'Street Address'),
    input('shippingAddress.line2', 'Apt/Suite Number'),
    input('shippingAddress.city', 'City'),
    input('shippingAddress.state', 'State'),
    input('shippingAddress.postalCode', 'Postal/ZIP Code'),
    input('shippingAddress.country', 'Choose a Country...', 'country'),
  ]
  events:
    "#{FormView.Events.SubmitFailed}": ()->
      requestAnimationFrame ()->
        $container = $(".error-container")
        if $container[0]
          $('html, body').animate(
            scrollTop: $container.offset().top-$(window).height()/2
          , 1000)
  js: (opts)->
    super

    @loading = true
    view = @view
    view.api = api = new Api opts.url, opts.token
    view.src = src = new Source
      name: view.path + '/' + opts.userId,
      path: view.path + '/' + opts.userId,
      api: api

    src.on Source.Events.LoadData, (model)=>
      @loading = false
      @model = model
      view.inputConfigs[1].hints += model.email
      view.initFormGroup.apply @
      riot.update()

  submit: ()->
    @api.patch(@src.path, @ctx.model)

new UserFormView

module.exports = UserFormView
