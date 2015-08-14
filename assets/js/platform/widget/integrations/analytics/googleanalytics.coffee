Integration = require '../integration'

class GoogleAnalytics extends Integration
  tag: 'ga-integration'
  html: ''
  img: window.staticUrl + '/img/integrations/google-analytics-logo.png'
  alt: 'Google Analytics'

GoogleAnalytics.register()

module.exports = GoogleAnalytics
