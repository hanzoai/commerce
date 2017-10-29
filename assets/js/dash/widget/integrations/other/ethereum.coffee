Integration = require '../integration'

input = require '../../../form/input'

class EthereumIntegrationForm extends Integration
  tag: 'ethereum-integration'
  type: 'ethereum'
  html: require '../../../templates/dash/widget/integrations/other/ethereum.html'
  img: '/img/integrations/ethereum.svg'
  text: 'Ethereum'
  alt: 'Ethereum'

  prefill: true
  duplicates: false

  inputConfigs: [
    input('data.address', 'Address',  'required')
    input('data.testAddress', 'TestAddress (optional)',  '')
  ]

EthereumIntegrationForm.register()

module.exports = EthereumIntegrationForm

