Page = require './page'

class Order extends Page
  tag: 'page-order'
  icon: 'glyphicon glyphicon-shopping-cart'
  name: 'Order'
  html: require '../../templates/backend/site/pages/order.html'

  collection: 'order'

Order.register()

module.exports = Order
