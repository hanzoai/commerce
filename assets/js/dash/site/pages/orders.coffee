Page = require './page'

class Orders extends Page
  tag: 'page-orders'
  icon: 'glyphicon glyphicon-shopping-cart'
  name: 'Orders'
  html: require '../../templates/dash/site/pages/orders.html'

  collection: 'orders'

Orders.register()

module.exports = Orders
