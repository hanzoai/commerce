promise = require 'bluebird'
riot = require 'riot'
util = require '../../util'

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
      switch name
        when 'period'
          @periodModel.value = value
        when 'date'
          @periodDateModel.value = value
      @refresh()

  chartModel:
    xAxis: [{
      categories: []
      crosshair: true
    }]
    yAxis: [
      {
        labels:
          format: '{value:.2f}'
          style:
            color: Highcharts.getOptions().colors[2]
        title:
          text: 'Sales',
          style:
              color: Highcharts.getOptions().colors[2]
      },
      {
        labels:
          style:
            color: Highcharts.getOptions().colors[0]
        title:
          text: 'Count',
          style:
              color: Highcharts.getOptions().colors[0]
        opposite: true
      },
    ]
    series: [
      {
        name: 'Sales'
        type: 'areaspline'
        data: []
        tooltip:
          valueSuffix: ' '
      }
      {
        name: 'Orders'
        type: 'spline'
        yAxis: 1
        data: []
        tooltip:
          valueSuffix: ' '
      }
      {
        name: 'Users'
        type: 'spline'
        yAxis: 1
        data: []
        tooltip:
          valueSuffix: ' '
      }
    ]

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
    @chartObs = {}
    riot.observable @chartObs

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
    compareYear = year = date.getFullYear()
    compareMonth = month = date.getMonth() + 1
    compareDay = day = date.getDate()

    percent = 1

    # calulate date intervals
    switch period
      when 'week'
        percent = (date.getDay() + 1) / 7
        compareDay -= 7
        @chartModel.xAxis[0].categories = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']

      when 'month'
        daysInMonth = lastDayInMonth()
        categories = []
        for d in [1..daysInMonth]
          categories.push "#{month}/#{d}"

        @chartModel.xAxis[0].categories = categories
        percent = day / daysInMonth
        compareMonth -= 1

    # Load new and comparative data from previous date interval
    promise.settle([
      @api.get("c/data/dashboard/#{period}ly/#{compareYear}/#{compareMonth}/#{compareDay}").then((res)=>
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

      largestCents = 0
      for currency, cents of @model.TotalSales
        if cents > largestCents && currency != ''
          @currency = currency
          largestCents = cents

      if @currency == ''
        super 0, 0, @currency
        return

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
      @totalSalesObs.trigger Events.Visual.NewData, @model.TotalSales[@currency], NaN, @currency
      @totalUsersObs.trigger Events.Visual.NewData, @model.TotalUsers, NaN
      @totalSubsObs.trigger Events.Visual.NewData, @model.TotalSubs, NaN

      @dailyOrdersObs.trigger Events.Visual.NewData, totalOrders, totalCompareOrders
      @dailySalesObs.trigger Events.Visual.NewData, totalCents[@currency], totalCompareCents[@currency], @currency
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

      @chartModel.series[0].data = @model.DailySales[@currency].map (val)=>
        return parseFloat(util.currency.renderUpdatedUICurrency '', val)
      @chartModel.series[1].data = @model.DailyOrders
      @chartModel.series[2].data = @model.DailyUsers

      @chartModel.yAxis[0].labels.format = "#{util.currency.getSymbol(@currency)}{value:.2f} (#{@currency.toUpperCase()})"

      @chartObs.trigger Events.Visual.NewData, @chartModel
    ).catch (e)=>
      console.log(e.stack)
      @error = e

Dashboard.register()

module.exports = Dashboard
