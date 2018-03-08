Integration = require '../integration'

input = require '../../../form/input'

class GoogleAnalytics extends Integration
  tag: 'ga-integration'
  type: 'analytics-google-analytics'
  html: require '../../../templates/dash/widget/integrations/analytics/ga.html'
  img: '/img/integrations/ga.png'
  alt: 'Google Analytics'
  text: 'Google Analytics'

  inputConfigs: [
    input('data.id', 'UA-XXXXXXXX-1', 'required')
    input('data.sampling', '', 'numeric')
  ]

GoogleAnalytics.register()

module.exports = GoogleAnalytics
