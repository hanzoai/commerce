Integration = require '../integration'

input = require '../../../form/input'

class FacebookPixel extends Integration
  tag: 'fb-pixel-integration'
  type: 'analytics-facebook-pixel'
  html: require '../../../templates/dash/widget/integrations/analytics/fbpixel.html'
  img: '/img/integrations/fb.png'
  text: 'Facebook Pixel'
  alt: 'Facebook Pixel'

  inputConfigs: [
    input('data.id', 'ex. 1234567890123', 'required')
    input('data.values.currency', '', 'currency-type-select')
    input('data.values.viewedProduct.percent', '', 'numeric')
    input('data.values.viewedProduct.value', '', 'money')
    input('data.values.addedProduct.percent', '', 'numeric')
    input('data.values.addedProduct.value', '', 'money')
    input('data.values.initiateCheckout.percent', '', 'numeric')
    input('data.values.initiateCheckout.value', '', 'money')
    input('data.values.addPaymentInfo.percent', '', 'numeric')
    input('data.values.addPaymentInfo.value', '', 'money')
    input('data.sampling', '', 'numeric')
  ]

FacebookPixel.register()

module.exports = FacebookPixel
