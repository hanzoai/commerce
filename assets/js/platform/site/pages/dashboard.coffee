Page = require './page'

class Dashboard extends Page
  tag: 'page-dashboard'
  icon: 'glyphicon glyphicon-home'
  name: 'Dashboard'
  html: require '../../templates/backend/site/pages/dashboard.html'

  collection: ''

Dashboard.register()

module.exports = Dashboard
