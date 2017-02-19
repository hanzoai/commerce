promise = require 'bluebird'
riot = require 'riot'
util = require '../../util'
store = require 'store'

Page = require './page'

capitalize = (str)->
  return str.charAt(0).toUpperCase() + str.slice(1)

lastDayInMonth = ()->
  date = new Date()
  return new Date(date.getFullYear(), date.getMonth() + 1, 0).getDate()

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events
Api = crowdcontrol.data.Api

# hack
currencyModel =
  name: 'currency'
  value: ''

class Dashboard extends Page
  tag: 'page-dashboard'
  icon: 'glyphicon glyphicon-home'
  name: 'Dashboard'
  html: require '../../templates/dash/site/pages/dashboard.html'

  collection: ''
  percent: 0

  events:
    "#{Events.Input.Change}": (name, value) ->
      switch name
        when 'period'
          @periodModel.value = value
          store.set 'periodModelValue', value
        when 'date'
          @periodDateModel.value = value
      @refresh()

  chartModel:
    type: 'line',
    data:
      labels: []
      datasets: [
        label: 'Sales',
        data: []
        pointBorderColor: '#1BE7FF'
        pointBackgroundColor: '#C0F8FF'
        borderColor: '#1BE7FF'
        yAxisID: 'Currency',
      ,
        label: 'Orders',
        data: []
        pointBorderColor: '#6EEB83'
        pointBackgroundColor: '#D7F9DD'
        borderColor: '#6EEB83'
        yAxisID: 'Count',
      ,
        label: 'Users',
        data: []
        pointBorderColor: '#FF5714'
        pointBackgroundColor: '#FFD1BE'
        borderColor: '#FF5714'
        yAxisID: 'Count',
      ]
    options:
      scales:
        xAxes: [
          type: 'category',
          position: 'bottom'
        ]
        yAxes: [
          id: 'Currency'
          type: 'linear'
          position: 'left'
          ticks:
            beginAtZero: true
            callback: (value)->
              v = parseInt(value * 100, 10) / 100
              ret = "#{util.currency.getSymbol(currencyModel.value)}#{v}"
              ret += " (#{currencyModel.value.toUpperCase()})" if currencyModel.value
              return ret
          scaleLabel:
            display: true
            labelString: 'Revenue'
        ,
          id: 'Count'
          type: 'linear'
          position: 'right'
          ticks:
            beginAtZero: true
          scaleLabel:
            display: true
            labelString: 'Amount'
        ]
      responsive: false

  # chartModel:
  #   xAxis: [{
  #     categories: []
  #     crosshair: true
  #   }]
  #   yAxis: [
  #     {
  #       legend:
  #         enabled: false
  #       floor: 0
  #       labels:
  #         format: '{value:.2f}'
  #         style:
  #           color: 'green'
  #       title:
  #         text: 'Sales',
  #         style:
  #             color: 'green'
  #     },
  #     {
  #       floor: 0
  #       labels:
  #         style:
  #           color: 'grey'
  #       title:
  #         text: 'Count',
  #         style:
  #             color: 'grey'
  #       opposite: true
  #     },
  #   ]
  #   series: [
  #     {
  #       name: 'Sales'
  #       type: 'areaspline'
  #       data: []
  #       tooltip:
  #         valueSuffix: ' '
  #     }
  #     {
  #       name: 'Orders'
  #       type: 'spline'
  #       yAxis: 1
  #       data: []
  #       tooltip:
  #         valueSuffix: ' '
  #     }
  #     {
  #       name: 'Users'
  #       type: 'spline'
  #       yAxis: 1
  #       data: []
  #       tooltip:
  #         valueSuffix: ' '
  #     }
  #   ]

  # For the period select
  currencyModel:
    name: 'currency'
    value: null

  periodModel:
    name: 'period'
    value: 'week'

  periodDateModel:
    name: 'date'
    value: ''

  currencyOptions: {}

  periodOptions:
    week: 'Week'
    month: 'Month'
    dai: 'Day'

  periodDescription: ()->
    return capitalize(@periodModel.value) + 'ly'

  periodLabel: ()->
    return 'THIS ' + if @periodModel.value == 'dai' then 'DAY' else @periodModel.value.toUpperCase()

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

    period = store.get 'periodModelValue'
    if period
      @periodModel.value = period

    @periodDateModel.value = "#{month}/#{day}/#{year}"

    @api = api = Api.get 'crowdstart'

    @on 'update', ()=>
      @updateCurrency()

    @refresh()

  refresh: ()->
    period = @periodModel.value
    date = new Date(@periodDateModel.value)
    compareYear = year = date.getFullYear()
    compareMonth = month = date.getMonth() + 1
    compareDay = day = date.getDate()

    @percent = 1

    # calulate date intervals
    switch period
      when 'dai'
        compareDay -=1
        d1 = new Date()
        if d1.getFullYear() == date.getFullYear() && d1.getMonth() == date.getMonth() && d1.getDate() == date.getDate()
          d2 = new Date(d1.getFullYear(), d1.getMonth(), d1.getDate(), 0,0,0)
          @percent = (d1.getTime() - d2.getTime()) / 8.64e+7
        @chartModel.data.labels = [
          '00:00'
          '01:00'
          '02:00'
          '03:00'
          '04:00'
          '05:00'
          '06:00'
          '07:00'
          '08:00'
          '09:00'
          '10:00'
          '11:00'
          '12:00'
          '13:00'
          '14:00'
          '15:00'
          '16:00'
          '17:00'
          '18:00'
          '19:00'
          '20:00'
          '21:00'
          '22:00'
          '23:00'
        ]

      when 'week'
        d1 = new Date()
        if d1.getFullYear() == date.getFullYear() && d1.getMonth() == date.getMonth() && d1.getDate() == date.getDate()
          @percent = (date.getDay() + 1) / 7
        compareDay -= 7
        @chartModel.data.labels = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']

      when 'month'
        daysInMonth = lastDayInMonth()
        categories = []
        for d in [1..daysInMonth]
          categories.push "#{month}/#{d}"

        @chartModel.data.labels = categories
        d1 = new Date()
        if d1.getFullYear() == date.getFullYear() && d1.getMonth() == date.getMonth()
          @percent = day / daysInMonth
        compareMonth -= 1

    tz = -date.getTimezoneOffset() / 60

    # Load new and comparative data from previous date interval
    promise.settle([
      @api.get("c/data/dashboard/#{period}ly/#{compareYear}/#{compareMonth}/#{compareDay}/#{tz}").then((res)=>
        @compareModel = res.responseText

        if res.status != 200 && res.status != 201 && res.status != 204
          throw new Error 'Form failed to load: '
      )

      @api.get("c/data/dashboard/#{period}ly/#{year}/#{month}/#{day}/#{tz}").then((res)=>
        @model = res.responseText

        if res.status != 200 && res.status != 201 && res.status != 204
          throw new Error 'Form failed to load: '
      )]
    ).then((rets)=>
      currencyModel = @currencyModel
      @currencyCount = 0
      for currency, cents of @model.TotalSales
        @currencyOptions[currency.toLowerCase()] = currency.toUpperCase()
        @currencyCount += 1
      largestCents = 0
      if !@currencyModel.value?
        for currency, cents of @model.TotalSales
          if cents > largestCents && currency != ''
            @currencyModel.value = currency
            largestCents = cents

        if @currencyModel.value == ''
          super 0, 0, @currencyModel.value
          return

      # Aggregate values for numeric panels
      totalOrders = 0
      totalCompareOrders = 0
      for orders, i in @model.DailyOrders
        totalOrders += orders
        totalCompareOrders += @compareModel.DailyOrders[i]

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

      @dailyOrdersObs.trigger Events.Visual.NewData, totalOrders, totalCompareOrders * @percent
      @dailyUsersObs.trigger Events.Visual.NewData, totalUsers, totalCompareUsers * @percent
      @dailySubsObs.trigger Events.Visual.NewData, totalSubs, totalCompareSubs * @percent

      @dailyOrdersObs.trigger Events.Visual.NewDescription, @periodDescription() + ' Orders'
      @dailySalesObs.trigger Events.Visual.NewDescription, @periodDescription() + ' Sales'
      @dailyUsersObs.trigger Events.Visual.NewDescription, @periodDescription() + ' Sign-ups'
      @dailySubsObs.trigger Events.Visual.NewDescription, @periodDescription() + ' Subscribers'

      @dailyOrdersObs.trigger Events.Visual.NewLabel, @periodLabel()
      @dailySalesObs.trigger Events.Visual.NewLabel, @periodLabel()
      @dailyUsersObs.trigger Events.Visual.NewLabel, @periodLabel()
      @dailySubsObs.trigger Events.Visual.NewLabel, @periodLabel()

      @update()
    ).catch (e)=>
      console.log(e.stack)
      @error = e

  updateCurrency: ()->
    if @model.DailySales?
      totalCents = {}
      totalCompareCents = {}
      for currency, values of @model.DailySales
        totalCents[currency] = 0
        totalCompareCents[currency] = 0
        for cents, i in values
          totalCents[currency] += cents
          if @compareModel.DailySales[currency]?
            totalCompareCents[currency] += @compareModel.DailySales[currency][i]
      @dailySalesObs.trigger Events.Visual.NewData, totalCents[@currencyModel.value], totalCompareCents[@currencyModel.value] * @percent, @currencyModel.value

    if @model.TotalSales?[@currencyModel.value]?
      @totalSalesObs.trigger Events.Visual.NewData, @model.TotalSales[@currencyModel.value], NaN, @currencyModel.value
    if @model.TotalUsers?
      @totalUsersObs.trigger Events.Visual.NewData, @model.TotalUsers, NaN
    if @model.TotalSubs?
      @totalSubsObs.trigger Events.Visual.NewData, @model.TotalSubs, NaN

    if @model.DailySales? && @model.DailyOrders?
      sales = @model.DailySales[@currencyModel.value]
      if sales?
        sales = @model.DailySales[@currencyModel.value].map (val)->
          return val/100
      else
        sales = @model.DailyOrders.map (val)->
          return 0

      salesXY = []
      for k, v of sales
        i = parseInt k, 10
        salesXY[i] =
          x: i
          y: v

      ordersXY = []
      for k, v of @model.DailyOrders
        i = parseInt k, 10
        ordersXY[i] =
          x: i
          y: v

      usersXY = []
      for k, v of @model.DailyUsers
        i = parseInt k, 10
        usersXY[i] =
          x: i
          y: v

      @chartModel.data.datasets[0].data = salesXY
      @chartModel.data.datasets[1].data = ordersXY
      @chartModel.data.datasets[2].data = usersXY

      # @chartModel.data.datasets[0].data = sales
      # @chartModel.data.datasets[1].data = @model.DailyOrders
      # @chartModel.data.datasets[2].data = @model.DailyUsers

      # @chartModel.yAxis[0].labels.format = "#{util.currency.getSymbol(@currencyModel.value)}{value:.2f} (#{@currencyModel.value.toUpperCase()})"

      @chartObs.trigger Events.Visual.NewData, @chartModel

Dashboard.register()

module.exports = Dashboard
