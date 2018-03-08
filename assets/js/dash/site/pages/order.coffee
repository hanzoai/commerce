Page = require './page'

class Order extends Page
  tag: 'page-order'
  icon: 'glyphicon glyphicon-shopping-cart'
  name: 'Order'
  html: require '../../templates/dash/site/pages/order.html'

  collection: 'order'

Order.register()

module.exports = Order
