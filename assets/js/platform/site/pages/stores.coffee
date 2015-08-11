Page = require './page'

class Stores extends Page
  tag: 'page-stores'
  icon: 'glyphicon glyphicon-home'
  name: 'Stores'
  html: require '../../templates/backend/site/pages/stores.html'

  collection: 'stores'

Stores.register()

module.exports = Stores
