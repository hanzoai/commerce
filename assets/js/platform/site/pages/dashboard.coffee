promise = require 'bluebird'
riot = require 'riot'

Page = require './page'

capitalize = (str)->
  return str.charAt(0).toUpperCase() + str.slice(1)

lastDayInMonth = ()->
  date = new Date()
  return new Date(date.getFullYear(), date.getMonth() + 1, 0).getDate()

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events
Api = crowdcontrol.data.Api

class Dashboard extends Page
  tag: 'page-dashboard'
  icon: 'glyphicon glyphicon-home'
  name: 'Dashboard'
  period: 'week'
  html: require '../../templates/backend/site/pages/dashboard.html'

  collection: ''

  periodDescription: ()->
    return capitalize(@period) + 'ly'

  periodLabel: ()->
    return 'THIS ' + @period.toUpperCase()

  js: (opts)->
    # Initialize communications
    @totalOrdersObs = {}
    @totalSalesObs = {}
    @dailyOrdersObs = {}
    @dailySalesObs = {}

    riot.observable @totalOrdersObs
    riot.observable @totalSalesObs
    riot.observable @dailyOrdersObs
    riot.observable @dailySalesObs

    # Date calculations
    period = @period

    date = new Date()
    year = date.getFullYear()
    month = date.getMonth() + 1
    day = date.getDate()

    percent = 1

    # calulate date intervals
    switch period
      when 'week'
        percent = (date.day + 1) / 7
      when 'month'
        percent = day / lastDayInMonth()

    @api = api = Api.get 'crowdstart'

    # Load new and comparative data from previous date interval
    promise.settle([
      api.get("c/data/dashboard/#{period}ly/#{year}/#{month}/#{day-7}").then((res)=>
        @compareModel = res.responseText

        if res.status != 200 && res.status != 201 && res.status != 204
          throw new Error 'Form failed to load: '
      )

      api.get("c/data/dashboard/#{period}ly/#{year}/#{month}/#{day}").then((res)=>
        @model = res.responseText

        if res.status != 200 && res.status != 201 && res.status != 204
          throw new Error 'Form failed to load: '
      )]
    ).then((rets)=>
      @currency = ''

      # Aggregate values for numeric panels
      totalOrders = 0
      totalCompareOrders = 0
      for orders, i in @model.DailyOrders
        totalOrders += orders
        totalCompareOrders += @compareModel.DailyOrders[i]

      totalCents = {}
      totalCompareCents = {}
      for currency, values of @model.DailySales
        totalCents[currency] = 0
        totalCompareCents[currency] = 0
        for cents, i in values
          totalCents[currency] += cents
          totalCompareCents[currency] += @compareModel.DailySales[currency][i]

      # Dispatch updated values
      @totalOrdersObs.trigger Events.Visual.NewData, @model.TotalOrders, NaN
      @totalSalesObs.trigger Events.Visual.NewData, @model.TotalSales, NaN

      @dailyOrdersObs.trigger Events.Visual.NewData, totalOrders, totalCompareOrders
      @dailySalesObs.trigger Events.Visual.NewData, totalCents, totalCompareCents
      @dailyOrdersObs.trigger Events.Visual.NewDescription, @periodDescription() + ' Orders'
      @dailySalesObs.trigger Events.Visual.NewDescription, @periodDescription() + ' Sales'
      @dailyOrdersObs.trigger Events.Visual.NewLabel, @periodLabel()
      @dailySalesObs.trigger Events.Visual.NewLabel, @periodLabel()
    ).catch (e)=>
      console.log(e.stack)
      @error = e

Dashboard.register()

module.exports = Dashboard
