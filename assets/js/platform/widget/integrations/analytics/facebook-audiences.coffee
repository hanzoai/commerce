Integration = require '../integration'

input = require '../../../form/input'

class FacebookAudiences extends Integration
  tag: 'fb-audiences-integration'
  type: 'facebook-audiences'
  html: require '../../../templates/backend/widget/integrations/analytics/fbaudiences.html'
  img: '/img/integrations/FB-f-Logo__blue_144.png'
  text: 'Facebook Audiences'
  alt: 'Facebook Audiences'

  inputConfigs: [
    input('id', 'ex. 1234567890123', 'required')
  ]

FacebookAudiences.register()

module.exports = FacebookAudiences
