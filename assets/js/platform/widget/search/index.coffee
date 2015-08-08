crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

utils = crowdcontrol.utils

View = crowdcontrol.view.View
Api = crowdcontrol.data.Api

class Search extends View
  tag: 'search-widget'
  html: require '../../templates/backend/widget/search/template.html'

  js: (opts)->
    super

    @userObs = {}
    utils.shim.observable @userObs

    @orderObs = {}
    utils.shim.observable @orderObs

    @api = api = Api.get 'platform'

    q = window.location.search

    api.get('search' + q).then((res) =>
      if res.status != 200 && res.status != 204
        throw new Error 'Form failed to load: '

      @model = model = res.responseText
      @userObs.trigger Events.Table.NewData, model.users
      @orderObs.trigger Events.Table.NewData, model.orders

      riot.update()
    ).catch (e)->
      console.log e.stack

Search.register()

module.exports = Search
