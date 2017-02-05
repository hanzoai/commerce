_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events
View = crowdcontrol.view.View

class Chart extends View
  tag: 'chart'
  html: ''
  title: ''
  subtitle: ''

  model: null
  ### model looks like:
    xAxis: [{ ...high chart options... }]
    yAxis: [{ ...high chart options... }]
    series: [{
      name: 'name'
      type: 'style'
      data: [ ...data points... ]
      ... other options ...
    }]
  ###

  events:
    "#{ Events.Visual.NewData }": (model)->
      @model = model
      @update()

  js: (opts)->
    @title = opts.title || @title
    @subtitle = opts.subtitle || @subtitle

    @on 'update', ()=>
      @refresh()

  refresh: ->
    @chart = new Highcharts.Chart
      credits: false
      chart:
        renderTo: @root
        zoomType: 'x'
      title:
        text: ''
      subtitle:
        text: @subtitle
      xAxis: @model.xAxis
      yAxis: @model.yAxis
      tooltip:
        shared: true
      legend:
        layout: 'vertical'
        align: 'right'
        x: -80
        verticalAlign: 'top'
        y: 55
        floating: true
        backgroundColor: (Highcharts.theme && Highcharts.theme.legendBackgroundColor) || '#FFFFFF'
      series: @model.series

    requestAnimationFrame ()=>
      @chart.reflow()

Highcharts.theme =
  colors: [
    '#DDDF0D'
    '#7798BF'
    '#55BF3B'
    '#DF5353'
    '#aaeeee'
    '#ff0066'
    '#eeaaee'
    '#55BF3B'
    '#DF5353'
    '#7798BF'
    '#aaeeee'
  ]
  chart:
    backgroundColor: 'white'
    borderWidth: 0
    borderRadius: 0
    plotBackgroundColor: null
    plotShadow: false
    plotBorderWidth: 0
  title: style:
    color: '#ddd;'
  subtitle: style:
    color: '#DDD'

  xAxis:
    labels: style:
      color: '#999'
    title: style:
      color: '#AAA'
    lineWidth: 0,
    minorGridLineWidth: 0,
    lineColor: 'transparent'

  yAxis:
    alternateGridColor: null
    minorTickInterval: null
    lineWidth: 0,
    minorGridLineWidth: 0,
    lineColor: 'transparent'
    labels: style:
      color: '#999'
    title: style:
      color: '#AAA'

  legend:
    enabled: false
    itemStyle: color: '#CCC'
    itemHoverStyle: color: '#FFF'
    itemHiddenStyle: color: '#333'
  labels: style: color: '#CCC'
  tooltip:
    backgroundColor:
      linearGradient:
        x1: 0
        y1: 0
        x2: 0
        y2: 1
      stops: [
        [
          0
          'rgba(96, 96, 96, .8)'
        ]
        [
          1
          'rgba(16, 16, 16, .8)'
        ]
      ]
    borderWidth: 0
    style: color: '#FFF'
  plotOptions:
    series: nullColor: '#444444'
    line:
      dataLabels: color: '#CCC'
      marker: lineColor: '#333'
    spline: marker: lineColor: '#333'
    scatter: marker: lineColor: '#333'
    candlestick: lineColor: 'white'
  toolbar: itemStyle: color: '#CCC'
  navigation: buttonOptions:
    symbolStroke: '#DDDDDD'
    hoverSymbolStroke: '#FFFFFF'
    theme:
      fill:
        linearGradient:
          x1: 0
          y1: 0
          x2: 0
          y2: 1
        stops: [
          [
            0.4
            '#606060'
          ]
          [
            0.6
            '#333333'
          ]
        ]
      stroke: '#000000'
  rangeSelector:
    buttonTheme:
      fill:
        linearGradient:
          x1: 0
          y1: 0
          x2: 0
          y2: 1
        stops: [
          [
            0.4
            '#888'
          ]
          [
            0.6
            '#555'
          ]
        ]
      stroke: '#000000'
      style:
        color: '#333'
      states:
        hover:
          fill:
            linearGradient:
              x1: 0
              y1: 0
              x2: 0
              y2: 1
            stops: [
              [
                0.4
                '#BBB'
              ]
              [
                0.6
                '#888'
              ]
            ]
          stroke: '#000000'
          style: color: 'white'
        select:
          fill:
            linearGradient:
              x1: 0
              y1: 0
              x2: 0
              y2: 1
            stops: [
              [
                0.1
                '#000'
              ]
              [
                0.3
                '#333'
              ]
            ]
          stroke: '#000000'
          style: color: 'yellow'
    inputStyle:
      backgroundColor: '#333'
      color: 'silver'
    labelStyle: color: 'silver'
  navigator:
    handles:
      backgroundColor: '#666'
      borderColor: '#AAA'
    outlineColor: '#CCC'
    maskFill: 'rgba(16, 16, 16, 0.5)'
    series:
      color: '#7798BF'
      lineColor: '#A6C7ED'
  scrollbar:
    barBackgroundColor: 'rgba(0,0,0,0.0)'
    barBorderColor: '#CCC'
    buttonArrowColor: '#CCC'
    buttonBackgroundColor:
      linearGradient:
        x1: 0
        y1: 0
        x2: 0
        y2: 1
      stops: [
        [
          0.4
          '#888'
        ]
        [
          0.6
          '#555'
        ]
      ]
    buttonBorderColor: '#CCC'
    rifleColor: '#FFF'
    trackBackgroundColor:
      linearGradient:
        x1: 0
        y1: 0
        x2: 0
        y2: 1
      stops: [
        [
          0
          '#000'
        ]
        [
          1
          '#333'
        ]
      ]
    trackBorderColor: '#666'
  legendBackgroundColor: 'rgba(48, 48, 48, 0.0)'
  background2: 'rgb(70, 70, 70)'
  dataLabelsColor: '#444'
  textColor: '#E0E0E0'
  maskColor: 'rgba(255,255,255,0.3)'

# Apply the theme
Highcharts.setOptions Highcharts.theme
Highcharts.setOptions
  chart:
    fontFamily: 'Open sans'

Chart.register()
module.exports = Chart
