Integration = require '../integration'

input = require '../../../form/input'

class RecaptchaIntegrationForm extends Integration
  tag: 'recaptcha-integration'
  type: 'recaptcha'
  html: require '../../../templates/dash/widget/integrations/other/recaptcha.html'
  img: '/img/integrations/recaptcha.png'
  text: 'Recaptcha'
  alt: 'Recaptcha'

  prefill: true
  duplicates: false

  inputConfigs: [
    input('data.secretKey', 'Secret Key',  'required')
  ]

RecaptchaIntegrationForm.register()

module.exports = RecaptchaIntegrationForm
