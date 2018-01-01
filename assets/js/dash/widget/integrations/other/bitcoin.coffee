Integration = require '../integration'

input = require '../../../form/input'

class BitcoinIntegrationForm extends Integration
  tag: 'bitcoin-integration'
  type: 'bitcoin'
  html: require '../../../templates/dash/widget/integrations/other/bitcoin.html'
  img: '/img/integrations/bitcoin.png'
  text: 'Bitcoin'
  alt: 'Bitcoin'

  prefill: true
  duplicates: false

  inputConfigs: [
    input('data.address', 'Address',  'required')
    input('data.testAddress', 'TestAddress',  'required')
  ]

BitcoinIntegrationForm.register()

module.exports = BitcoinIntegrationForm

