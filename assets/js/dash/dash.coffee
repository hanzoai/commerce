riot         = require 'riot'
_            = require 'underscore'
crowdcontrol = require 'crowdcontrol'

window.riot   = riot
window.moment = require 'moment'

window.crowdstart =
  form:   require './form'
  site:   require './site'
  table:  require './table'
  util:   require './util'
  visual: require './visual'
  widget: require './widget'
