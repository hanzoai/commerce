Integration = require '../integration'

input = require '../../../form/input'

class GoogleAnalytics extends Integration
  tag: 'ga-integration'
  type: 'google-analytics'
  html: require '../../../templates/backend/widget/integrations/analytics/ga.html'
  img: '/img/integrations/ga.png'
  alt: 'Google Analytics'
  text: 'Google Analytics'

  inputConfigs: [
    input('id', 'UA-XXXXXXXX-1', 'required')
    input('sampling', '', 'numeric')
  ]

GoogleAnalytics.register()

module.exports = GoogleAnalytics
