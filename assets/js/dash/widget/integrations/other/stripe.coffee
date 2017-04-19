Integration = require '../integration'

input = require '../../../form/input'

class StripeIntegrationForm extends Integration
  tag: 'st-ripe-integration'
  type: 'stripe'
  html: require '../../../templates/dash/widget/integrations/other/stripe.html'
  img: '/img/integrations/stripe.png'
  text: 'Stripe'
  alt: 'Stripe'

  prefill: true
  duplicates: false

  inputConfigs: []

StripeIntegrationForm.register()

module.exports = StripeIntegrationForm
