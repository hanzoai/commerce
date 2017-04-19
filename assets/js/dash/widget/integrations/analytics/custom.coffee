Integration = require '../integration'

input = require '../../../form/input'

class CustomAnalytics extends Integration
  tag: 'custom-integration'
  type: 'analytics-custom'
  html: require '../../../templates/dash/widget/integrations/analytics/custom.html'
  img: '/img/integrations/custom.png'
  text: 'Custom Analytics'
  alt: 'Custom Analytics'
  model:
    code: '//Do Something in JS'

  inputConfigs: [
    input('data.event', 'Choose an event', 'analytics-events-select required')
    input('data.code', '//Do Something in JS', 'js required')
    input('data.sampling', '', 'numeric')
  ]

CustomAnalytics.register()

module.exports = CustomAnalytics
