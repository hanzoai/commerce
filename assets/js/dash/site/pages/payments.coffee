Page = require './page'

class Payments extends Page
  tag: 'page-payments'
  icon: 'fa fa-money'
  name: 'Payments'
  html: require '../../templates/backend/site/pages/payments.html'

  collection: 'payments'

Payments.register()

module.exports = Payments
