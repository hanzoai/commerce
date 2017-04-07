_ = require 'underscore'

input = require '../input'
Form = require './form'

class RecaptchaIntegrationForm extends Form
  # break the tr because stupid regex in riot
  tag: 'recaptcha-integration-form'
  path: 'integration/recaptcha'

  prefill: true

  inputConfigs: [
    input('secretKey', 'Secret Key',  'required')
    input('enabled', 'Enabled',  'switch')
  ]

RecaptchaIntegrationForm.register()

module.exports = RecaptchaIntegrationForm
