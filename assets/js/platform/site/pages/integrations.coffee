Page = require './page'

class Integrations extends Page
  tag: 'page-integrations'
  icon: 'fa fa-credit-card'
  name: 'Integrations'
  html: require '../../templates/backend/site/pages/integrations.html'

  collection: 'integrations'

Integrations.register()

module.exports = Integrations
