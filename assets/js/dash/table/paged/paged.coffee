_ = require 'underscore'
crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

store = require 'store'
table = require '../types'

Api = crowdcontrol.data.Api
BasicTableView = table.BasicTableView
m = crowdcontrol.utils.mediator

lowerCaseFirstLetter = (string) ->
  return string.charAt(0).toLowerCase() + string.slice(1)

capitalizeFirstLetter = (string) ->
  return string.charAt(0).toUpperCase() + string.slice(1)

getSortField = (sortField)->
  if sortField == "Id"
    return "Id_"

  return sortField

class BasicPagedTable extends BasicTableView
  tag: 'basic-paged-table'
  html: require '../../templates/dash/table/paged/template.html'
  page: 1
  maxPage: 2
  display: 10
  $pagination: $()
  loaded: false

  # Lock the form controls when paging/loading
  lock: false

  # Is the data being sourced from a static model?
  # If this is set to the array of records from which to source model records from
  staticModel: null

  events:
    "#{Events.Table.PrepareForNewData}": ()->
      @loaded = false
      @staticModel = null
      @model = []
      @lock = true
      @update()

    "#{Events.Table.NewData}": (model)->
      @loaded = true
      @staticModel = model
      @loadData()

    # finishing a form that is linked to this table will refresh it
    "#{Events.Form.SubmitSuccess}": ()->
      setTimeout ()=>
        @refresh()
      , 1000

  js: (opts)->
    super

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

    @on 'update', ()=>
      requestAnimationFrame ()=>
        @initDynamicContent()

    @refresh()

  sort: (id)->
    field = capitalizeFirstLetter id
    return ()=>
      if @locked
        return

      if @headerMap[id]?.hints['dontsort']
        return

      if field == 'Number'
        # field = '__key__'
        return
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
          if page != @page && !@lock
            @lock = true
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
    sortField = getSortField @filterModel.sortField

    if @staticModel?
      # sort the static model if we have static model to sort

      # fields on json are camelcase while the ones in query are capitalized
      sortField = lowerCaseFirstLetter sortField

      coeff = 1
      if @filterModel.sortDirection == 'sort-asc'
        coeff = -1

      @staticModel.sort((a, b)->
        if _.isNumber(a[sortField])
          return coeff *(a[sortField] - b[sortField])
        else if moment.isDate(a[sortField])
          return  coeff *(if moment(a[sortField]).isAfter(b[sortField]) then 1 else -1)
        else if _.isString(a[sortField])
          return  coeff *(a[sortField].localeCompare b[sortField])
        else if _.isBoolean(a[sortField])
          return  coeff *(if a[sortField] then 1 else -1)
        else
          return -coeff
      )

      @loadData()
      return

    # construct sort query string if querying server
    path = @path + '?page=' + @page + '&display=' + @display + '&sort=' + (if @filterModel.sortDirection == 'sort-desc' then '' else '-') + sortField
    path += '&limit=1000' if !window.User.owner
    requestAnimationFrame ()->
      $('.previous, .next').addClass('disabled')

    @api.get(path).then (res) =>
      @loaded = true
      data = res.responseText

      @loadData(data)

  # This handles
  loadData: (data)->
    # riot maintainer, why do you allow for desync in 2.2.x
    @model = []
    @update()

    if @staticModel?
      @model = @staticModel.slice @display * (@page - 1), @display * @page
      @count = @staticModel.length
    else
      @model = data.models
      @count = data.count

    @maxPage = Math.ceil @count/(data?.display ? @display)

    riot.update()

    @initDynamicContent()
    @$pagination.jqPagination 'option', 'max_page', @maxPage

    @lock = false

    requestAnimationFrame ()->
      $('.previous, .next').removeClass('disabled')

module.exports = BasicPagedTable
