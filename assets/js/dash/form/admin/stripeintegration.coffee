_ = require 'underscore'

input = require '../input'
Form = require './form'

class StripeIntegrationForm extends Form
  # break the tr because stupid regex in riot
  tag: 'st-ripe-integration-form'
  path: 'stripe'

  prefill: true

  _submit:()->

  stripeOAuth: ()->
    window.location.href = "https://connect.stripe.com/oauth/authorize?response_type=code&client_id=#{ @model.clientId }&scope=read_write&state=movetoserver&stripe_landing=login&redirect_uri=#{ @model.redirectUrl }"

  stripeSync: (event)->
    @api.get('stripe/sync')
    $(event.target).html 'Syncing...'
    @syncing = true

StripeIntegrationForm.register()

module.exports = StripeIntegrationForm
