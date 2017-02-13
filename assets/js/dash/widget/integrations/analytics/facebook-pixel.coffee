Integration = require '../integration'

input = require '../../../form/input'

class FacebookPixel extends Integration
  tag: 'fb-pixel-integration'
  type: 'facebook-pixel'
  html: require '../../../templates/dash/widget/integrations/analytics/fbpixel.html'
  img: '/img/integrations/fb.png'
  text: 'Facebook Pixel'
  alt: 'Facebook Pixel'

  inputConfigs: [
    input('id', 'ex. 1234567890123', 'required')
    input('values.currency', '', 'currency-type-select')
    input('values.viewedProduct.percent', '', 'numeric')
    input('values.viewedProduct.value', '', 'money')
    input('values.addedProduct.percent', '', 'numeric')
    input('values.addedProduct.value', '', 'money')
    input('values.initiateCheckout.percent', '', 'numeric')
    input('values.initiateCheckout.value', '', 'money')
    input('values.addPaymentInfo.percent', '', 'numeric')
    input('values.addPaymentInfo.value', '', 'money')
    input('sampling', '', 'numeric')
  ]

FacebookPixel.register()

module.exports = FacebookPixel
