riot = require 'riot'
_ = require 'underscore'

window.riot = riot

crowdcontrol = require 'crowdcontrol'

_.mixin deepExtend: require('underscore-deep-extend')(_)

window.moment = require 'moment'
window.crowdstart =
  table: require    './table'
  form: require     './form'
  widget: require   './widget'
  util: require     './util'
