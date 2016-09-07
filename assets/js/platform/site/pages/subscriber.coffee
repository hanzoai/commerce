Page = require './page'

class Subscriber extends Page
  tag: 'page-subscriber'
  icon: 'fa fa-newspaper-o'
  name: 'Subscriber'
  html: require '../../templates/backend/site/pages/subscriber.html'

  collection: 'subscriber'

Subscriber.register()

module.exports = Subscriber
