Integration = require '../integration'

input = require '../../../form/input'

class MailchimpIntegrationForm extends Integration
  tag: 'mailchimp-integration'
  type: 'mailchimp'
  html: require '../../../templates/dash/widget/integrations/other/mailchimp.html'
  img: '/img/integrations/mailchimp.png'
  text: 'Mailchimp'
  alt: 'Mailchimp'

  prefill: true
  duplicates: false

  inputConfigs: [
    input('data.listId', 'List Id',  'required')
    input('data.apiKey', 'API Key',  'required')
  ]

MailchimpIntegrationForm.register()

module.exports = MailchimpIntegrationForm
