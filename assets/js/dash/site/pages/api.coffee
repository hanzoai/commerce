Page = require './page'

class Api extends Page
  tag: 'page-api'
  icon: 'fa fa-key'
  name: 'API'
  html: require '../../templates/backend/site/pages/api.html'

  collection: 'api'

Api.register()

module.exports = Api
