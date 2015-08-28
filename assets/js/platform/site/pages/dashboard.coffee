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
  html: require '../../templates/backend/site/pages/dashboard.html'

  collection: ''

  events:
    "#{Events.Input.Change}": (name, value) ->
      @refresh()

  # For the period select
  periodModel:
    name: 'period'
    value: 'week'

  periodDateModel:
    name: 'date'
    value: ''

  periodOptions:
    week: 'Week'
    month: 'Month'

  periodDescription: ()->
    return capitalize(@periodModel.value) + 'ly'

  periodLabel: ()->
    return 'THIS ' + @periodModel.value.toUpperCase()

  js: (opts)->
    # Initialize communications
    @totalOrdersObs = {}
    @totalSalesObs = {}
    @totalUsersObs = {}
    @totalSubsObs = {}

    riot.observable @totalOrdersObs
    riot.observable @totalSalesObs
    riot.observable @totalUsersObs
    riot.observable @totalSubsObs

    @dailyOrdersObs = {}
    @dailySalesObs = {}
    @dailyUsersObs = {}
    @dailySubsObs = {}

    riot.observable @dailyOrdersObs
    riot.observable @dailySalesObs
    riot.observable @dailyUsersObs
    riot.observable @dailySubsObs

    # Date calculations
    date = new Date()
    year = date.getFullYear()
    month = date.getMonth() + 1
    day = date.getDate()

    @periodDateModel.value = "#{month}/#{day}/#{year}"

    @api = api = Api.get 'crowdstart'

    @refresh()

  refresh: ()->
    period = @periodModel.value
    date = new Date(@periodDateModel.value)
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


    # Load new and comparative data from previous date interval
    promise.settle([
      @api.get("c/data/dashboard/#{period}ly/#{year}/#{month}/#{day-7}").then((res)=>
        @compareModel = res.responseText

        if res.status != 200 && res.status != 201 && res.status != 204
          throw new Error 'Form failed to load: '
      )

      @api.get("c/data/dashboard/#{period}ly/#{year}/#{month}/#{day}").then((res)=>
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

      totalUsers = 0
      totalCompareUsers = 0
      for users, i in @model.DailyUsers
        totalUsers += users
        totalCompareUsers += @compareModel.DailyUsers[i]

      totalSubs = 0
      totalCompareSubs = 0
      for subs, i in @model.DailySubscribers
        totalSubs += subs
        totalCompareSubs += @compareModel.DailySubscribers[i]

      # Dispatch updated values
      @totalOrdersObs.trigger Events.Visual.NewData, @model.TotalOrders, NaN
      @totalSalesObs.trigger Events.Visual.NewData, @model.TotalSales, NaN
      @totalUsersObs.trigger Events.Visual.NewData, @model.TotalUsers, NaN
      @totalSubsObs.trigger Events.Visual.NewData, @model.TotalSubs, NaN

      @dailyOrdersObs.trigger Events.Visual.NewData, totalOrders, totalCompareOrders
      @dailySalesObs.trigger Events.Visual.NewData, totalCents, totalCompareCents
      @dailyUsersObs.trigger Events.Visual.NewData, totalUsers, totalCompareUsers
      @dailySubsObs.trigger Events.Visual.NewData, totalSubs, totalCompareSubs

      @dailyOrdersObs.trigger Events.Visual.NewDescription, @periodDescription() + ' Orders'
      @dailySalesObs.trigger Events.Visual.NewDescription, @periodDescription() + ' Sales'
      @dailyUsersObs.trigger Events.Visual.NewDescription, @periodDescription() + ' Sign-ups'
      @dailySubsObs.trigger Events.Visual.NewDescription, @periodDescription() + ' Subscribers'

      @dailyOrdersObs.trigger Events.Visual.NewLabel, @periodLabel()
      @dailySalesObs.trigger Events.Visual.NewLabel, @periodLabel()
      @dailyUsersObs.trigger Events.Visual.NewLabel, @periodLabel()
      @dailySubsObs.trigger Events.Visual.NewLabel, @periodLabel()
    ).catch (e)=>
      console.log(e.stack)
      @error = e

Dashboard.register()

module.exports = Dashboard
