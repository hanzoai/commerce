Page = require './page'

class Order extends Page
  tag: 'page-orders'
  icon: 'glyphicon glyphicon-shopping-cart'
  name: 'Order'
  html: require '../../templates/backend/site/pages/orders.html'

  collection: 'order'

Order.register()

module.exports = Order
