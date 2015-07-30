riot = require 'riot'

routes = {}

module.exports = Router =
  add: (collection, action = '', clas)->
    routes[collection + '_' + action] = clas

  Menu: undefined
  Crumbs: undefined

lastPages = null

changePage = (collection = '', id = '', action = '') ->
  page = routes[collection + '_' + action]
  if page?
    if lastPages?
      for lastPage in lastPages
        lastPage.unmount()

    Router.Menu.setActive(page)
    Router.Crumbs.setActive(page)

    proto = page.prototype

    $('#content .tray').html "<#{proto.tag}/>"
    lastPages = riot.mount proto.tag, _id: id

riot.route changePage

$ ()->
  hash = window.location.hash.replace('#','')

  if hash == ''
    changePage()
  else
    riot.route hash
