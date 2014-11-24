ModelEmitter = require 'mvstar/lib/model-emitter'

validNum = (v) ->
  typeof v is 'number'

neverBelowZero = (v) ->
  if v < 0 then 0 else v

cookies =
  get: (name) ->
    try
      state = (JSON.parse $.cookie name) ? {}
    catch
      {}

  set: (name, state, path, expires) ->
    $.cookie name, (JSON.stringify state),
      path:    path
      expires: expires

class Cart extends ModelEmitter
  cookieName: 'SKULLYCart'

  defaults:
    subtotal: 0
    quantity: 0
    products: {}

  validators:
    quantity: validNum
    subtotal: validNum

  transforms:
    quantity: neverBelowZero
    subtotal: neverBelowZero

  fetch: ->
    @update cookies.get @cookieName

  save: ->
    cookies.set @cookieName, @state, '/', 30

  set: (k, v) ->
    @emit k, v
    super

  getProduct: (sku) ->
    @state.products[sku]

  getProducts: ->
    @state.products

  setProduct: (sku, product) ->
    @state.products[sku] = product
    @save()

  addProduct: (sku, product) ->
    # update quantity and subtotal
    quantity = @get 'quantity'
    subtotal = @get 'subtotal'
    @set 'quantity', quantity + product.quantity
    @set 'subtotal', subtotal + (product.quantity * product.price)

    # save new item to cart
    unless @state.products[sku]?
      return @setProduct sku, product

    # update quantity of product in cart
    @state.products[sku].quantity += product.quantity
    @save()

  removeProduct: (sku) ->
    product = @state.products[sku]

    quantity = @get 'quantity'
    subtotal = @get 'subtotal'
    @set 'quantity', quantity - product.quantity
    @set 'subtotal', subtotal - (product.quantity * product.price)

    delete @state.products[sku]
    @save()

  clear: ->
    $.removeCookie @cookieName
    @state = {}
    @setDefaults()
    @set 'quantity', 0
    @set 'subtotal', 0
    @save()

module.exports = Cart
