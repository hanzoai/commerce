riot = require 'riot'
window.riot = riot

crowdcontrol = require 'crowdcontrol'

window.moment = require 'moment'
window.crowdstart =
  views: require './views'
  form: require './form'
  data: crowdcontrol.data
