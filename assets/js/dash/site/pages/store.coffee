Page = require './page'

class Store extends Page
  tag: 'page-store'
  icon: 'glyphicon glyphicon-home'
  name: 'Store'
  html: require '../../templates/dash/site/pages/store.html'

  collection: 'store'

Store.register()

module.exports = Store
