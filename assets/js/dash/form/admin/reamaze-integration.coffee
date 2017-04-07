_ = require 'underscore'

input = require '../input'
Form = require './form'

class ReamazeIntegrationForm extends Form
  # break the tr because stupid regex in riot
  tag: 'reamaze-integration-form'
  path: 'integration/reamaze'

  prefill: true

  inputConfigs: [
    input('secret', 'Secret',  'required')
  ]

ReamazeIntegrationForm.register()

module.exports = ReamazeIntegrationForm
