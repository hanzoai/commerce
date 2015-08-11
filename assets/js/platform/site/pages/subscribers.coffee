Page = require './page'

class Subscribers extends Page
  tag: 'page-subscribers'
  icon: 'fa fa-newspaper-o'
  name: 'Subscribers'
  html: require '../../templates/backend/site/pages/subscribers.html'

  collection: 'subscribers'

Subscribers.register()

module.exports = Subscribers
