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
        try
          lastPage.unmount()
        catch e

    Router.Menu.setActive(page)
    Router.Crumbs.setActive(page)
    Router.Crumbs.setActiveId(id)

    proto = page.prototype

    $('#content').html '<div id="replaceme"/>'
    lastPages = riot.mount '#replaceme', proto.tag, _id: id

    $(window).scrollTop(0)

riot.route changePage

$ ()->
  hash = window.location.hash.replace('#','')

  if hash == ''
    changePage()
  else
    riot.route hash
