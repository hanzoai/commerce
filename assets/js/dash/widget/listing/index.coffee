_ = require 'underscore'
crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

util = require '../../util'
table = require '../../table'

input = require '../../form/input'

Api = crowdcontrol.data.Api
View = crowdcontrol.view.View

BasicTableView = table.BasicTableView

class ListingWidget extends View
  tag: 'listing-widget'
  html: require '../../templates/dash/widget/listing/template.html'
  events:
    "#{Events.Form.Prefill}": (model)->
      @model = model
      @model.listing = {} if !@model.listing?
      @reset()

  js: (opts)->
    super

    @util = util

    @api = api = Api.get 'crowdstart'

    api.get('product').then((res)=>
      if res.status != 200
        throw new Error 'Reference Products failed to load'

      @products = res.responseText.models
      @reset()

    ).catch (e)->
      console.log e.stack

  reset: ()->
    @listings = []
    if @products?
      for product in @products
        matched = false
        if @model?.listings?
          for productId, listing of @model.listings
            if productId == product.id
              @listings.unshift
                productId:    product.id
                slug:         product.slug
                price:        listing.price ? product.price
                listPrice:    listing.listPrice ? product.listPrice
                available:    listing.available ? product.available
                show:         true
              matched = true
              break

        if !matched
          @listings.push
            productId:  product.id
            slug:       product.slug
            price:      product.price
            listPrice:  product.listPrice
            available:  false
            show:       false

      @updateModel()

  updateModel: ()->
    @model.listings = listings = {}
    for listing in @listings
      listings[listing.productId] = listing if listing.show

    riot.update()

  currency: ()->
    return @model.currency

  changePrice: (i)->
    return (event)=>
      val = $(event.target).val()
      @listings[i].price = util.currency.renderJSONCurrencyFromUI(@currency(), val)

  changeListPrice: (i)->
    return (event)=>
      val = $(event.target).val()
      @listings[i].listPrice = util.currency.renderJSONCurrencyFromUI(@currency(), val)

  changeAvailable: (i)->
    return (event)=>
      val = event.target.checked
      @listings[i].available = val

      # An in stock item must be included in the stoer
      if val
        @changeShow(i)(event)

  changeShow: (i)->
    return (event)=>
      val = event.target.checked
      @listings[i].show = val

      # An item cannot be in stock if it does not belong to the store
      if !val
        @changeAvailable(i)(event)
      @updateModel()

ListingWidget.register()

