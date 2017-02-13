Integration = require '../integration'

input = require '../../../form/input'

class FacebookConversions extends Integration
  tag: 'fb-conversions-integration'
  type: 'facebook-conversions'
  html: require '../../../templates/dash/widget/integrations/analytics/fbconversions.html'
  img: '/img/integrations/fb.png'
  text: 'Facebook Conversions'
  alt: 'Facebook Analytics'

  inputConfigs: [
    input('id', 'ex. 1234567890123', 'required')
    input('event', 'Choose an event', 'analytics-events-select required')
    input('sampling', '', 'numeric')
  ]

FacebookConversions.register()

module.exports = FacebookConversions
