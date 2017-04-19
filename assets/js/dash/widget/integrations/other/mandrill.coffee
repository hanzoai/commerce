Integration = require '../integration'

input = require '../../../form/input'

class MandrillIntegrationForm extends Integration
  tag: 'mandrill-integration'
  type: 'mandrill'
  html: require '../../../templates/dash/widget/integrations/other/mandrill.html'
  img: '/img/integrations/mandrill.png'
  text: 'Mandrill'
  alt: 'Mandrill'

  prefill: true
  duplicates: false

  inputConfigs: [
    input('data.apiKey', 'API Key',  'required')
  ]

MandrillIntegrationForm.register()

module.exports = MandrillIntegrationForm
