_ = require 'underscore'

input = require '../input'
Form = require './form'

class MailchimpIntegrationForm extends Form
  # break the tr because stupid regex in riot
  tag: 'mailchimp-integration-form'
  path: 'integration/mailchimp'

  prefill: true

  inputConfigs: [
    input('listId', 'List Id',  'required')
    input('apiKey', 'API Key',  'required')
  ]

MailchimpIntegrationForm.register()

module.exports = MailchimpIntegrationForm
