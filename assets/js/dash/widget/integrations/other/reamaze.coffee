Integration = require '../integration'

input = require '../../../form/input'

class ReamazeIntegrationForm extends Integration
  tag: 'reamaze-integration'
  type: 'reamaze'
  html: require '../../../templates/dash/widget/integrations/other/reamaze.html'
  img: '/img/integrations/reamaze.png'
  text: 'Reamaze'
  alt: 'Reamaze'

  prefill: true
  duplicates: false

  inputConfigs: [
    input('data.secret', 'Secret',  'required')
  ]

ReamazeIntegrationForm.register()

module.exports = ReamazeIntegrationForm
