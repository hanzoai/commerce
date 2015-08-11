_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

View = crowdcontrol.view.View
Router = require '../router'

class Page extends View
  tag: 'page'

  # page data
  icon: 'fa fa-circle-thin'
  name: 'Page'
  crumbs: [
    # array of Page derived classes
  ]

  # route
  collection: ''
  _id: ''
  action: ''

  # methods
  render: ()->

  js: (opts)->
    super
    $(@root).attr('id', 'current-page').addClass('animated').addClass('fadeIn')
    @render()

  @register: ()->
    Router.add @prototype.collection, @prototype.action, @
    super

module.exports = Page
