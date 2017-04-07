_ = require 'underscore'

input = require '../input'
Form = require './form'

class MandrillIntegrationForm extends Form
  # break the tr because stupid regex in riot
  tag: 'mandrill-integration-form'
  path: 'integration/mandrill'

  prefill: true

  inputConfigs: [
    input('apiKey', 'API Key',  'required')
  ]

MandrillIntegrationForm.register()

module.exports = MandrillIntegrationForm
