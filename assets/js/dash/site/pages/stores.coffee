Page = require './page'

class Stores extends Page
  tag: 'page-stores'
  icon: 'glyphicon glyphicon-home'
  name: 'Stores'
  html: require '../../templates/dash/site/pages/stores.html'

  collection: 'stores'

Stores.register()

module.exports = Stores
