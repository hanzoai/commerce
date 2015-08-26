promise = require 'bluebird'
riot = require 'riot'

Page = require './page'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events
Api = crowdcontrol.data.Api

class Dashboard extends Page
  tag: 'page-dashboard'
  icon: 'glyphicon glyphicon-home'
  name: 'Dashboard'
  html: require '../../templates/backend/site/pages/dashboard.html'

  collection: ''

  js: (opts)->
    @totalOrdersObs = {}
    @totalSalesObs = {}
    @dailySalesObs = {}

    riot.observable @totalOrdersObs
    riot.observable @totalSalesObs
    riot.observable @dailySalesObs

    period = 'weekly'

    date = new Date()
    year = date.getFullYear()
    month = date.getMonth() + 1
    day = date.getDate()

    weeklyPercent = (date.day + 1) / 7

    @api = api = Api.get 'crowdstart'

    promise.settle([
      api.get("c/data/dashboard/#{period}/#{year}/#{month}/#{day-7}").then((res)=>
        @compareModel = res.responseText

        if res.status != 200 && res.status != 201 && res.status != 204
          throw new Error 'Form failed to load: '
      )

      api.get("c/data/dashboard/#{period}/#{year}/#{month}/#{day}").then((res)=>
        @model = res.responseText

        if res.status != 200 && res.status != 201 && res.status != 204
          throw new Error 'Form failed to load: '
      )]
    ).then((rets)=>
      @currency = ''

      totalCents = {}
      totalCompareCents = {}
      for currency, values of @model.DailySales
        totalCents[currency] = 0
        totalCompareCents[currency] = 0
        for cents, i in values
          totalCents[currency] += cents
          totalCompareCents[currency] += @compareModel.DailySales[currency][i]

      @totalOrdersObs.trigger Events.Visual.NewData, @model.TotalOrders, 0
      @totalSalesObs.trigger Events.Visual.NewData, @model.TotalSales, 0

      @dailySalesObs.trigger Events.Visual.NewData, totalCents, totalCompareCents

    ).catch (e)=>
      console.log(e.stack)
      @error = e

Dashboard.register()

module.exports = Dashboard
