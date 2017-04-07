_ = require 'underscore'

input = require '../input'
Form = require './form'

class NetlifyIntegrationForm extends Form
  # break the tr because stupid regex in riot
  tag: 'netlify-integration-form'
  path: 'integration/netlify'

  prefill: true

  inputConfigs: [
    input('accessToken', 'Access Token',  'required')
  ]

NetlifyIntegrationForm.register()

module.exports = NetlifyIntegrationForm
