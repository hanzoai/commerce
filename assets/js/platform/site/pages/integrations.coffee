Page = require './page'

Integration = require '../../widget/integrations'

class Integrations extends Page
  tag: 'page-integrations'
  icon: 'fa fa-credit-card'
  name: 'Integrations'
  html: require '../../templates/backend/site/pages/integrations.html'

  type: 'paymentprocessors'

  setType: (t)->
    return (e)=>
      @type = t
      e.preventDefault()

  collection: 'integrations'

  analyticsIntegrations: [
    Integration.Analytics.GoogleAnalytics
  ]

  js: ()->
    super

    @on 'update', ()->
      $('#current-page').css
        'padding-bottom': '20px'

Integrations.register()

module.exports = Integrations
