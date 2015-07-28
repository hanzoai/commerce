_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

View = crowdcontrol.view.View

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
  id: ''
  action: ''

  render: ()->

  js: (opts)->
    super
    $(@root).attr('id', 'current-page')
    @render()

  @register: ()->
    super


