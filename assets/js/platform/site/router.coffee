riot = require 'riot'

routes = {}

module.exports = Router =
  add: (collection, action = '', clas)->
    routes[collection + '_' + action] = clas

riot.route (collection, id, action) ->
  page = routes[collection + '_' + action]
  if page?
    proto = page.prototype
    $('#content').html "<#{proto.tag}/>"
    riot.mount proto.tag, id: id
