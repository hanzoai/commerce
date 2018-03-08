Page = require './page'

class Payment extends Page
  tag: 'page-payment'
  icon: 'fa fa-money'
  name: 'Payments'
  html: require '../../templates/dash/site/pages/payment.html'

  collection: 'payment'

Payment.register()

module.exports = Payment
