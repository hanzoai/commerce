Integration = require '../integration'

input = require '../../../form/input'

class FacebookPixel extends Integration
  tag: 'fb-pixel-integration'
  type: 'facebook-pixel'
  html: require '../../../templates/backend/widget/integrations/analytics/fbpixel.html'
  img: '/img/integrations/fb.png'
  text: 'Facebook Pixel'
  alt: 'Facebook Pixel'

  inputConfigs: [
    input('id', 'ex. 1234567890123', 'required')
  ]

FacebookPixel.register()

module.exports = FacebookPixel
