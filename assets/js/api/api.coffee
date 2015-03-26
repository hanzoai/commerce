require 'crowdstart.js/src/index'

order = require './order'

$('.charge').click (e)->
  Crowdstart.charge order, (status, data, loc) ->
    console.log status, data
    if loc?
      window.location = loc

$('.authorize').click (e)->
  Crowdstart.authorize order, (status, data, loc) ->
    console.log status, data
    if loc?
      window.location = loc
