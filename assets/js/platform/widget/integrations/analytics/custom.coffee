Integration = require '../integration'

input = require '../../../form/input'

class CustomAnalytics extends Integration
  tag: 'custom-integration'
  type: 'custom'
  html: require '../../../templates/backend/widget/integrations/analytics/custom.html'
  img: '/img/integrations/custom.png'
  text: 'Custom Analytics'
  alt: 'Custom Analytics'
  model:
    code: '//Do Something in JS'

  inputConfigs: [
    input('event', 'Choose an event', 'analytics-events-select required')
    input('code', '//Do Something in JS', 'js required')
    input('sampling', '', 'numeric')
  ]

CustomAnalytics.register()

module.exports = CustomAnalytics
