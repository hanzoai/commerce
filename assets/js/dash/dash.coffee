window._      = require 'underscore'
window.moment = require 'moment'
window.riot   = require 'riot'

window.crowdstart =
  form:   require './form'
  site:   require './site'
  table:  require './table'
  util:   require './util'
  visual: require './visual'
  widget: require './widget'
