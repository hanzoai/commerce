Integration = require '../integration'

input = require '../../../form/input'

class GoogleAdwords extends Integration
  tag: 'gadwords-integration'
  type: 'google-adwords'
  html: require '../../../templates/backend/widget/integrations/analytics/gadwords.html'
  img: '/img/integrations/adwrds.png'
  alt: 'Google Adwords'
  text: 'Google Adwords'

  inputConfigs: [
    input('id', '123456789', 'required')
    input('event', 'Choose an event', 'analytics-events-select required')
    input('sampling', '', 'numeric')
  ]

GoogleAdwords.register()

module.exports = GoogleAdwords
