# we require table events so require tables first
require '../../table'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

utils = crowdcontrol.utils

View = crowdcontrol.view.View
Api = crowdcontrol.data.Api

class Search extends View
  tag: 'search-widget'
  html: require '../../templates/dash/widget/search/template.html'

  js: (opts)->
    super

    @userObs = {}
    utils.shim.observable @userObs

    @orderObs = {}
    utils.shim.observable @orderObs

    @api = api = Api.get 'crowdstart'

    q = window.location.search
    q += '&limit=1000&page=1&display=1000' if !window.User.owner

    requestAnimationFrame ()=>
      @userObs.trigger Events.Table.StartSearch
      @orderObs.trigger Events.Table.StartSearch
      riot.update()

    api.get('order' + q).then((res) =>
      if res.status != 200 && res.status != 204
        throw new Error 'Form failed to load: '

      @model = model = res.responseText
      @orderObs.trigger Events.Table.NewData, model.models

      @orderObs.trigger Events.Table.EndSearch

      riot.update()
    ).catch (e)->
      console.log e.stack

    api.get('user' + q).then((res) =>
      if res.status != 200 && res.status != 204
        throw new Error 'Form failed to load: '

      @model = model = res.responseText
      @userObs.trigger Events.Table.NewData, model.models

      @userObs.trigger Events.Table.EndSearch

      riot.update()
    ).catch (e)->
      console.log e.stack

Search.register()

module.exports = Search
