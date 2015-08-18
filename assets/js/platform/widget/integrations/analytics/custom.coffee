Integration = require '../integration'

input = require '../../../form/input'

class CustomAnalytics extends Integration
  tag: 'custom-integration'
  type: 'generic'
  html: require '../../../templates/backend/widget/integrations/analytics/custom.html'
  img: '/img/integrations/custom-logo.png'
  text: 'Custom Analytics'
  alt: 'Custom Analytics'
  model:
    code: '//Do Something in JS'

  inputConfigs: [
    input('event', 'Choose an event', 'analytics-events-dropdown required')
    input('code', '//Do Something in JS', 'js required')
  ]

CustomAnalytics.register()

module.exports = CustomAnalytics
