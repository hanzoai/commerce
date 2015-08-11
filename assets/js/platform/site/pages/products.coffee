Page = require './page'

class Products extends Page
  tag: 'page-products'
  icon: 'glyphicon glyphicon-book'
  name: 'Products'
  html: require '../../templates/backend/site/pages/products.html'

  collection: 'products'

Products.register()

module.exports = Products
