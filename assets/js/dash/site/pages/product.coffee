Page = require './page'

class Product extends Page
  tag: 'page-product'
  icon: 'glyphicon glyphicon-book'
  name: 'Product'
  html: require '../../templates/backend/site/pages/product.html'

  collection: 'product'

Product.register()

module.exports = Product
