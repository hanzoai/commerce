Page = require './page'

class Search extends Page
  tag: 'page-search'
  icon: 'fa fa-search'
  name: 'Search'
  html: require '../../templates/backend/site/pages/search.html'

  collection: 'search'

Search.register()

module.exports = Search
