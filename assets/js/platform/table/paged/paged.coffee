crowdcontrol = require 'crowdcontrol'

table = require '../types'

Api = crowdcontrol.data.Api
BasicTableView = table.BasicTableView
m = crowdcontrol.utils.mediator

capitalizeFirstLetter = (string) ->
  return string.charAt(0).toUpperCase() + string.slice(1)

class BasicPagedTable extends BasicTableView
  tag: 'basic-paged-table'
  html: require './template.html'
  page: 1
  maxPage: 2
  display: 10
  $pagination: $()
  sortField: 'UpdatedAt'
  sortDirection: ''
  firstLoad: false
  js: (opts)->
    @path = opts.path if opts.path
    @api = Api.get 'crowdstart'

    m.trigger 'start-spin', @tag + @path + '-paged-table-load'

    @on 'update', ()=>
      requestAnimationFrame ()=>
        @initDynamicContent()

    @refresh()

  sort: (id)->
    field = capitalizeFirstLetter id
    return ()=>
      if field != @sortField
        @sortField = field
        @sortDirection = 'sort-desc'
      else if @sortDirection != 'sort-desc'
        @sortDirection = 'sort-desc'
      else
        @sortDirection = 'sort-asc'
      @refresh()

  initDynamicContent: ()->
    $select = $($(@root).find('select')[0])
    if !@initializedSelect && $select[0]?
      $select.select2(
        minimumResultsForSearch: Infinity
      ).change (event)=>@updateDisplay(event)
      @initializedSelect = true

    @$pagination = $pagination = $(@root).find('.pagination')
    if !@initializedPaging && $pagination[0]?
      $pagination.jqPagination
        paged: (page)=>
          if page != @page
            @page = page
            @refresh()
      @initializedPaging = true

    requestAnimationFrame ()=>
      if @initializedSelect
        $select.select2('val', @display)

  updateDisplay: (event)->
    display = parseInt $(event.target).val(), 10
    if @display != display
      @display = display
      @refresh()

  refresh: ()->
    path = @path + '?page=' + @page + '&display=' + @display + '&sort=' + (if @sortDirection == 'sort-desc' then '' else '-') + if @sortField == "Id" then "Id_" else @sortField
    @api.get(path).then (res) =>
      @firstLoad = true

      m.trigger 'stop-spin', @tag + @path + '-paged-table-load'
      data = res.responseText
      @model = data.models

      @maxPage = Math.ceil data.count/data.display

      @update()

      @initDynamicContent()
      @$pagination.jqPagination 'option', 'max_page', @maxPage

module.exports = BasicPagedTable
