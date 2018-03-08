Integration = require '../integration'

input = require '../../../form/input'

class GoogleAdwords extends Integration
  tag: 'gadwords-integration'
  type: 'analytics-google-adwords'
  html: require '../../../templates/dash/widget/integrations/analytics/gadwords.html'
  img: '/img/integrations/adwrds.png'
  alt: 'Google Adwords'
  text: 'Google Adwords'

  inputConfigs: [
    input('data.id', '123456789', 'required')
    input('data.event', 'Choose an event', 'analytics-events-select required')
    input('data.sampling', '', 'numeric')
  ]

GoogleAdwords.register()

module.exports = GoogleAdwords
