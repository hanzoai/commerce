Integration = require '../integration'

input = require '../../../form/input'

class NetlifyIntegrationForm extends Integration
  tag: 'netlify-integration'
  type: 'netlify'
  html: require '../../../templates/dash/widget/integrations/other/netlify.html'
  img: '/img/integrations/netlify.png'
  text: 'Netlify'
  alt: 'Netlify'

  prefill: true
  duplicates: false

  inputConfigs: [
    input('data.accessToken', 'Access Token',  'required')
  ]

NetlifyIntegrationForm.register()

module.exports = NetlifyIntegrationForm

