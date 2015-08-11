crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

store = require 'store'
table = require '../types'

Api = crowdcontrol.data.Api
BasicTableView = table.BasicTableView
m = crowdcontrol.utils.mediator

capitalizeFirstLetter = (string) ->
  return string.charAt(0).toUpperCase() + string.slice(1)

class BasicPagedTable extends BasicTableView
  tag: 'basic-paged-table'
  html: require '../../templates/backend/table/paged/template.html'
  page: 1
  maxPage: 2
  display: 10
  $pagination: $()
  firstLoad: false
  pagingLock: false

  events:
    # finishing a form that is linked to this table will refresh it
    "#{Events.Form.SubmitSuccess}": ()->
      setTimeout ()=>
        @refresh()
      , 1000

  js: (opts)->
    display = store.get 'display'
    if display
      @display = display
    else
      store.set 'display', @display

    @filterModel =
      sortField: 'UpdatedAt'
      sortDirection: ''
      minDate: ''
      maxDate: ''

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
      if @headerMap[id].hints['dontsort']
        return

      if field == 'Number'
        field = '__key__'
      if field != @filterModel.sortField
        @filterModel.sortField = field
        @filterModel.sortDirection = 'sort-desc'
      else if @filterModel.sortDirection != 'sort-desc'
        @filterModel.sortDirection = 'sort-desc'
      else
        @filterModel.sortDirection = 'sort-asc'
      @refresh()

  initDynamicContent: ()->
    $select = $($(@root).find('select')[0])
    if $select[0]?
      if !@initializedSelect
        $select.select2(
          minimumResultsForSearch: Infinity
        ).change (event)=>@updateDisplay(event)
        @initializedSelect = true
      else
        setTimeout ()=>
          $select.select2('val', @display)
        , 500

    @$pagination = $pagination = $(@root).find('.pagination')
    if !@initializedPaging && $pagination[0]?
      $pagination.jqPagination
        paged: (page)=>
          if page != @page && !@pagingLock
            @pagingLock = true
            @page = page
            @refresh()
      @initializedPaging = true

  updateDisplay: (event)->
    display = parseInt $(event.target).val(), 10
    if @display != display
      store.set 'display', display
      @display = display
      @page = 1
      @refresh()

      requestAnimationFrame ()=>
        @initDynamicContent()

  refresh: ()->
    path = @path + '?page=' + @page + '&display=' + @display + '&sort=' + (if @filterModel.sortDirection == 'sort-desc' then '' else '-') + if @filterModel.sortField == "Id" then "Id_" else @filterModel.sortField
    requestAnimationFrame ()->
      $('.previous, .next').addClass('disabled')

    @api.get(path).then (res) =>
      @firstLoad = true

      m.trigger 'stop-spin', @tag + @path + '-paged-table-load'
      data = res.responseText
      @model = data.models
      @count = data.count

      @maxPage = Math.ceil data.count/data.display

      @update()

      @initDynamicContent()
      @$pagination.jqPagination 'option', 'max_page', @maxPage

      @pagingLock = false

      requestAnimationFrame ()->
        $('.previous, .next').removeClass('disabled')

module.exports = BasicPagedTable
